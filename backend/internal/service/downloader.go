package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/config"
)

// Downloader 下载器接口
type Downloader interface {
	AddTorrent(ctx context.Context, torrentURL, savePath string) (string, error)
	PauseTorrent(ctx context.Context, torrentID string) error
	ResumeTorrent(ctx context.Context, torrentID string) error
	RemoveTorrent(ctx context.Context, torrentID string, removeFiles bool) error
	GetTorrentInfo(ctx context.Context, torrentID string) (map[string]interface{}, error)
}

// DownloaderException 下载器异常
type DownloaderException struct {
	Message string
}

func (e *DownloaderException) Error() string {
	return e.Message
}

// QBittorrentDownloader qBittorrent 下载器实现
type QBittorrentDownloader struct {
	cfg     *config.Config
	client  *http.Client
	sid     string
	sidMu   sync.Mutex
}

func NewQBittorrentDownloader(cfg *config.Config) *QBittorrentDownloader {
	return &QBittorrentDownloader{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (q *QBittorrentDownloader) ensureLogin(ctx context.Context) error {
	q.sidMu.Lock()
	defer q.sidMu.Unlock()

	if q.sid != "" {
		return nil
	}

	loginURL := strings.TrimRight(q.cfg.DownloaderHost, "/") + "/api/v2/auth/login"
	form := url.Values{
		"username": {q.cfg.DownloaderUsername},
		"password": {q.cfg.DownloaderPassword},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("创建登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接 qBittorrent 失败: %w", err)
	}
	defer resp.Body.Close()

	// 提取 SID cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			q.sid = cookie.Value
			break
		}
	}

	if q.sid == "" {
		return &DownloaderException{Message: "qBittorrent 登录失败：未获取 SID"}
	}

	zap.L().Info("qBittorrent 登录成功")
	return nil
}

func (q *QBittorrentDownloader) apiURL(path string) string {
	return strings.TrimRight(q.cfg.DownloaderHost, "/") + "/api/v2" + path
}

func (q *QBittorrentDownloader) doRequest(ctx context.Context, method, apiPath string, body io.Reader) (*http.Response, error) {
	if err := q.ensureLogin(ctx); err != nil {
		return nil, err
	}

	reqURL := q.apiURL(apiPath)
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{Name: "SID", Value: q.sid})

	return q.client.Do(req)
}

func (q *QBittorrentDownloader) AddTorrent(ctx context.Context, torrentURL, savePath string) (string, error) {
	form := url.Values{
		"urls":      {torrentURL},
		"savepath":  {savePath},
	}

	resp, err := q.doRequest(ctx, http.MethodPost, "/torrents/add", strings.NewReader(form.Encode()))
	if err != nil {
		return "", &DownloaderException{Message: fmt.Sprintf("添加种子失败: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &DownloaderException{Message: fmt.Sprintf("添加种子失败，状态码: %d", resp.StatusCode)}
	}

	// 等待种子出现在列表中
	var hash string
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		hash, err = q.findTorrentHash(ctx, torrentURL)
		if err == nil && hash != "" {
			break
		}
	}

	if hash == "" {
		return "", &DownloaderException{Message: "添加种子后未找到对应的 hash"}
	}

	return hash, nil
}

func (q *QBittorrentDownloader) findTorrentHash(ctx context.Context, torrentURL string) (string, error) {
	resp, err := q.doRequest(ctx, http.MethodGet, "/torrents/info", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var torrents []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return "", err
	}

	for _, t := range torrents {
		if h, ok := t["hash"].(string); ok {
			return h, nil
		}
	}
	return "", fmt.Errorf("未找到种子")
}

func (q *QBittorrentDownloader) PauseTorrent(ctx context.Context, torrentID string) error {
	form := url.Values{"hashes": {torrentID}}
	resp, err := q.doRequest(ctx, http.MethodPost, "/torrents/pause", strings.NewReader(form.Encode()))
	if err != nil {
		return &DownloaderException{Message: fmt.Sprintf("暂停种子失败: %v", err)}
	}
	defer resp.Body.Close()
	return nil
}

func (q *QBittorrentDownloader) ResumeTorrent(ctx context.Context, torrentID string) error {
	form := url.Values{"hashes": {torrentID}}
	resp, err := q.doRequest(ctx, http.MethodPost, "/torrents/resume", strings.NewReader(form.Encode()))
	if err != nil {
		return &DownloaderException{Message: fmt.Sprintf("恢复种子失败: %v", err)}
	}
	defer resp.Body.Close()
	return nil
}

func (q *QBittorrentDownloader) RemoveTorrent(ctx context.Context, torrentID string, removeFiles bool) error {
	form := url.Values{
		"hashes":      {torrentID},
		"deleteFiles": {fmt.Sprintf("%v", removeFiles)},
	}
	resp, err := q.doRequest(ctx, http.MethodPost, "/torrents/delete", strings.NewReader(form.Encode()))
	if err != nil {
		return &DownloaderException{Message: fmt.Sprintf("删除种子失败: %v", err)}
	}
	defer resp.Body.Close()
	return nil
}

func (q *QBittorrentDownloader) GetTorrentInfo(ctx context.Context, torrentID string) (map[string]interface{}, error) {
	resp, err := q.doRequest(ctx, http.MethodGet, fmt.Sprintf("/torrents/info?hashes=%s", torrentID), nil)
	if err != nil {
		return nil, &DownloaderException{Message: fmt.Sprintf("获取种子信息失败: %v", err)}
	}
	defer resp.Body.Close()

	var torrents []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, err
	}

	if len(torrents) == 0 {
		return nil, &DownloaderException{Message: "种子不存在"}
	}

	t := torrents[0]
	result := make(map[string]interface{})

	if progress, ok := t["progress"].(float64); ok {
		result["progress"] = progress
	}
	if downloaded, ok := t["downloaded"].(float64); ok {
		result["downloaded_bytes"] = int64(downloaded)
	}
	if total, ok := t["size"].(float64); ok {
		result["total_bytes"] = int64(total)
	}
	if speed, ok := t["dlspeed"].(float64); ok {
		result["download_speed"] = int64(speed)
	}
	if eta, ok := t["eta"].(float64); ok {
		result["eta"] = int(eta)
	}

	state := "downloading"
	if s, ok := t["state"].(string); ok {
		switch s {
		case "uploading", "stalledUP", "queuedUP":
			state = "completed"
		case "pausedDL", "pausedUP":
			state = "paused"
		case "error", "missingFiles", "unknown":
			state = "error"
		default:
			state = "downloading"
		}
	}
	result["status"] = state

	return result, nil
}
