package download

import (
	"context"
	"fmt"
	"strings"

	"github.com/anidog/anidog-go/internal/service"
)

// publicTrackers 是一组高活跃度的公共 BT tracker，入队前会注入到不带 &tr= 参数
// 的 magnet 链接中。
//
// 为什么需要：Mikan / 部分 RSS 给的 magnet 是裸 hash（仅 xt + dn），qBit 拿到后
// 只能靠 DHT/PeX/LSD 自己找 peer。当 swarm 里完整源恰好掉线时，连接率会塌得很快。
// 注入公共 tracker 后能多一条独立的 peer 发现通道，对"DHT 残骸还在但当前没人完整
// 做种"的情况救活率显著提升。
//
// 选取标准：opentrackr / openbittorrent / 知名长寿节点 + udp 优先（开销最低）。
// 数量控制在 ~15 条以内，更多 tracker 反而会拖慢 announce。
var publicTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.tracker.cl:1337/announce",
	"udp://open.demonii.com:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"udp://exodus.desync.com:6969/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"udp://tracker-udp.gbitt.info:80/announce",
	"udp://explodie.org:6969/announce",
	"udp://tracker.tiny-vps.com:6969/announce",
	"udp://oh.fuuuuuck.com:6969/announce",
	"http://tracker.opentrackr.org:1337/announce",
	"http://open.acgnxtracker.com/announce",
	"http://share.camoe.cn:8080/announce",
	"http://t.acg.rip:6699/announce",
	"http://tracker.bt4g.com:2095/announce",
}

// EnrichMagnetWithTrackers 给 magnet 链接追加公共 tracker。
//
// 行为：
//   - 非 magnet 链接（http/https/.torrent）原样返回
//   - magnet 已含任何 &tr= 参数 → 信任原始 tracker，不注入（避免破坏发布者意图）
//   - magnet 仅有 xt+dn 等基础参数 → 追加 publicTrackers
//
// 单元可测：对外暴露给 indexer / 手动选种等所有最终把 URL 喂给 qBit 的入口。
func EnrichMagnetWithTrackers(rawURL string) string {
	if !strings.HasPrefix(rawURL, "magnet:") {
		return rawURL
	}
	// 已经带 tracker 就别动
	if strings.Contains(rawURL, "&tr=") || strings.Contains(rawURL, "?tr=") {
		return rawURL
	}
	var b strings.Builder
	b.Grow(len(rawURL) + 32*len(publicTrackers))
	b.WriteString(rawURL)
	for _, t := range publicTrackers {
		b.WriteString("&tr=")
		// magnet 的 tr 参数标准做法是 URL-encode；qBit/绝大多数客户端
		// 都接受未 encode 的形式（uri 里 : / 是合法字符），但为稳妥起见
		// 还是按规范 encode。
		b.WriteString(percentEncodeTracker(t))
	}
	return b.String()
}

// percentEncodeTracker 对 tracker URL 做最小化的 percent-encoding，只编码
// magnet query 里有歧义的字符。比 url.QueryEscape 更克制（保留 :/?#=& 中只编码
// 真正会破坏解析的 & 和空格）。
func percentEncodeTracker(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ':' || c == '/' || c == '.' || c == '-' || c == '_' || c == '~':
			b.WriteByte(c)
		case (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xF])
		}
	}
	return b.String()
}

// TorrentExecutor executes torrent downloads via qBittorrent.
type TorrentExecutor struct {
	client service.Downloader
}

// NewTorrentExecutor creates a new torrent executor.
func NewTorrentExecutor(client service.Downloader) *TorrentExecutor {
	return &TorrentExecutor{client: client}
}

// Execute adds a torrent to qBittorrent and returns the hash.
func (e *TorrentExecutor) Execute(ctx context.Context, task *Task, progressCB ProgressCallback) (*Result, error) {
	savePath := ""
	if task.SavePath != "" {
		savePath = task.SavePath
	}

	url := EnrichMagnetWithTrackers(task.URL)

	hash, err := e.client.AddTorrent(ctx, url, savePath)
	if err != nil {
		return nil, fmt.Errorf("添加种子失败: %w", err)
	}

	return &Result{TorrentID: hash}, nil
}

// Cancel removes the torrent from qBittorrent.
func (e *TorrentExecutor) Cancel(taskID string) error {
	return e.client.RemoveTorrent(context.Background(), taskID, true)
}

// Pause pauses the torrent in qBittorrent.
func (e *TorrentExecutor) Pause(taskID string) error {
	return e.client.PauseTorrent(context.Background(), taskID)
}

// Resume resumes the torrent in qBittorrent.
func (e *TorrentExecutor) Resume(taskID string) error {
	return e.client.ResumeTorrent(context.Background(), taskID)
}

// Remove removes the torrent from qBittorrent.
func (e *TorrentExecutor) Remove(taskID string, removeFiles bool) error {
	return e.client.RemoveTorrent(context.Background(), taskID, removeFiles)
}
