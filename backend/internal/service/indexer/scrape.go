package indexer

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// UDP tracker scrape (BEP 15) —— 用来在不下载的情况下查询 magnet 当前
// seeders/leechers 数。准确率比 indexer 自报的更高，因为是直接问 tracker。
//
// 设计目标：
//   - 输入 magnet 或 InfoHash + trackers 列表
//   - 并发查询，3s 超时
//   - 取所有 tracker 回包中 seeders 最大值（最乐观估计）
//   - 任何 tracker 不通 / 超时不阻塞整体

// ScrapeResult 单次 scrape 结果
type ScrapeResult struct {
	InfoHash  string
	Seeders   int
	Leechers  int
	Completed int
	OK        bool   // true = 至少一个 tracker 给了响应
	Note      string // 诊断信息
}

// 默认公共 UDP tracker 池 —— 当 magnet 自带 tracker 全死时兜底
var defaultUDPTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.tracker.cl:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"udp://exodus.desync.com:6969/announce",
}

// ScrapeMagnet 对一个 magnet 或 InfoHash 做 scrape。
// magnetOrHash 可以是：
//   - 完整 magnet URL（"magnet:?xt=urn:btih:HASH&tr=...&tr=..."）
//   - 40 字符 hex InfoHash
//   - 32 字符 base32 InfoHash
// 当 magnet 不带 tracker 时回退到 defaultUDPTrackers。
func ScrapeMagnet(ctx context.Context, magnetOrHash string) ScrapeResult {
	infoHash, trackers := parseMagnet(magnetOrHash)
	if infoHash == "" {
		return ScrapeResult{Note: "无法解析 InfoHash"}
	}
	hashBytes, err := hex.DecodeString(infoHash)
	if err != nil || len(hashBytes) != 20 {
		return ScrapeResult{InfoHash: infoHash, Note: "InfoHash 长度无效"}
	}

	// 仅保留 udp:// tracker —— http(s) tracker 协议复杂，暂不支持
	udpTrackers := make([]string, 0, len(trackers))
	for _, t := range trackers {
		if strings.HasPrefix(strings.ToLower(t), "udp://") {
			udpTrackers = append(udpTrackers, t)
		}
	}
	if len(udpTrackers) == 0 {
		udpTrackers = append(udpTrackers, defaultUDPTrackers...)
	}

	// 并发查询所有 udp tracker，取最大 seeders 的回包
	type single struct {
		seeders, leechers, completed int
		ok                           bool
	}
	results := make(chan single, len(udpTrackers))
	var wg sync.WaitGroup
	for _, tr := range udpTrackers {
		wg.Add(1)
		go func(trackerURL string) {
			defer wg.Done()
			s, l, c, err := udpScrape(ctx, trackerURL, hashBytes)
			if err == nil {
				results <- single{s, l, c, true}
			} else {
				results <- single{}
			}
		}(tr)
	}
	go func() { wg.Wait(); close(results) }()

	best := single{}
	count := 0
	for r := range results {
		if r.ok {
			count++
			if r.seeders > best.seeders {
				best = r
			}
		}
	}

	return ScrapeResult{
		InfoHash:  infoHash,
		Seeders:   best.seeders,
		Leechers:  best.leechers,
		Completed: best.completed,
		OK:        count > 0,
		Note:      fmt.Sprintf("已查询 %d 个 tracker，%d 个回包", len(udpTrackers), count),
	}
}

// parseMagnet 从 magnet URL 提取 InfoHash 和 tracker 列表。
// 输入也可以是裸 hex/base32 InfoHash。
func parseMagnet(s string) (infoHash string, trackers []string) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(strings.ToLower(s), "magnet:") {
		// 当作裸 InfoHash：40 hex 或 32 base32
		clean := strings.TrimSpace(s)
		if len(clean) == 40 {
			return strings.ToLower(clean), nil
		}
		if len(clean) == 32 {
			// base32 → hex
			h := base32ToHex(clean)
			if h != "" {
				return h, nil
			}
		}
		return "", nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", nil
	}
	q := u.Query()
	for _, xt := range q["xt"] {
		if strings.HasPrefix(xt, "urn:btih:") {
			h := strings.TrimPrefix(xt, "urn:btih:")
			if len(h) == 40 {
				infoHash = strings.ToLower(h)
			} else if len(h) == 32 {
				infoHash = base32ToHex(h)
			}
			break
		}
	}
	trackers = q["tr"]
	return infoHash, trackers
}

// base32ToHex BitTorrent 的 base32 InfoHash 是 RFC 4648 (uppercase, no padding)
func base32ToHex(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	s = strings.ToUpper(s)
	if len(s) != 32 {
		return ""
	}
	bits := make([]byte, 0, 160)
	for _, c := range s {
		idx := strings.IndexRune(alphabet, c)
		if idx < 0 {
			return ""
		}
		for i := 4; i >= 0; i-- {
			bits = append(bits, byte((idx>>i)&1))
		}
	}
	out := make([]byte, 20)
	for i := 0; i < 20; i++ {
		var b byte
		for j := 0; j < 8; j++ {
			b = (b << 1) | bits[i*8+j]
		}
		out[i] = b
	}
	return hex.EncodeToString(out)
}

// udpScrape 执行 BEP 15 UDP tracker scrape：
//   1. connect 请求（4s 超时）拿 connection_id
//   2. scrape 请求（3s 超时）拿 seeders/completed/leechers
func udpScrape(ctx context.Context, trackerURL string, infoHash []byte) (seeders, leechers, completed int, err error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("解析 tracker URL: %w", err)
	}
	host := u.Host
	if host == "" {
		return 0, 0, 0, errors.New("tracker 缺少 host")
	}

	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", host)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("拨号失败: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(3 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	conn.SetDeadline(deadline)

	// === connect ===
	transactionID := rand.Uint32()
	connectReq := make([]byte, 16)
	binary.BigEndian.PutUint64(connectReq[0:8], 0x41727101980) // protocol_id
	binary.BigEndian.PutUint32(connectReq[8:12], 0)            // action=connect
	binary.BigEndian.PutUint32(connectReq[12:16], transactionID)

	if _, err := conn.Write(connectReq); err != nil {
		return 0, 0, 0, fmt.Errorf("connect 发送: %w", err)
	}
	connectResp := make([]byte, 16)
	if _, err := conn.Read(connectResp); err != nil {
		return 0, 0, 0, fmt.Errorf("connect 接收: %w", err)
	}
	action := binary.BigEndian.Uint32(connectResp[0:4])
	respTID := binary.BigEndian.Uint32(connectResp[4:8])
	if action != 0 || respTID != transactionID {
		return 0, 0, 0, errors.New("connect 响应无效")
	}
	connectionID := binary.BigEndian.Uint64(connectResp[8:16])

	// === scrape ===
	transactionID = rand.Uint32()
	scrapeReq := make([]byte, 16+20)
	binary.BigEndian.PutUint64(scrapeReq[0:8], connectionID)
	binary.BigEndian.PutUint32(scrapeReq[8:12], 2) // action=scrape
	binary.BigEndian.PutUint32(scrapeReq[12:16], transactionID)
	copy(scrapeReq[16:36], infoHash)

	if _, err := conn.Write(scrapeReq); err != nil {
		return 0, 0, 0, fmt.Errorf("scrape 发送: %w", err)
	}
	scrapeResp := make([]byte, 8+12)
	n, err := conn.Read(scrapeResp)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("scrape 接收: %w", err)
	}
	if n < 20 {
		return 0, 0, 0, fmt.Errorf("scrape 响应过短: %d", n)
	}
	respAction := binary.BigEndian.Uint32(scrapeResp[0:4])
	respTID2 := binary.BigEndian.Uint32(scrapeResp[4:8])
	if respAction != 2 || respTID2 != transactionID {
		return 0, 0, 0, errors.New("scrape 响应无效")
	}
	seeders = int(binary.BigEndian.Uint32(scrapeResp[8:12]))
	completed = int(binary.BigEndian.Uint32(scrapeResp[12:16]))
	leechers = int(binary.BigEndian.Uint32(scrapeResp[16:20]))
	return
}
