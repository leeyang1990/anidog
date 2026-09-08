package qbittorrent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/downloader"
	"go.uber.org/zap"
)

type QBittorrent struct {
	client    *http.Client
	config    *Config
	baseURL   string
	sessionID string
}

func NewProvider(cfg *config.Config) (downloader.TorrentEngine, error) {
	q := &QBittorrent{
		config: NewConfig(
			cfg.DownloaderHost,
			cfg.DownloaderUsername,
			cfg.DownloaderPassword,
		),
		baseURL: strings.TrimSuffix(cfg.DownloaderHost, "/"),
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("创建 cookie jar 失败: %w", err)
	}
	q.client = &http.Client{Jar: jar}

	// 登录
	if err := q.login(); err != nil {
		return nil, fmt.Errorf("qBittorrent 登录失败: %w", err)
	}

	zap.L().Info("qBittorrent provider 初始化成功",
		zap.String("host", q.baseURL),
	)

	return q, nil
}

func (q *QBittorrent) login() error {
	data := url.Values{}
	data.Set("username", q.config.Username)
	data.Set("password", q.config.Password)

	// qBittorrent 要求有 Referer 头（防 CSRF）
	req, err := http.NewRequest("POST", q.baseURL+"/api/v2/auth/login", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", q.baseURL)

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("登录失败 status=%d: %s", resp.StatusCode, string(body))
	}
	// 不同 qBittorrent 版本会返回 200 + "Ok." 或 204 空响应。
	// 只要是 2xx 且正文没有明确 Fails 就视为成功。
	if strings.Contains(string(body), "Fails") {
		return fmt.Errorf("登录失败: %s（检查 username/password）", string(body))
	}

	// 从 Set-Cookie 读取 SID（替代 Jar.Cookies(nil) 的 nil URL panic）
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			q.sessionID = cookie.Value
			break
		}
	}

	return nil
}

func (q *QBittorrent) AddTorrent(ctx context.Context, torrentURL, savePath string) (string, error) {
	body, status, err := q.doAddTorrent(ctx, torrentURL, savePath)
	// 403 通常意味着 session 失效（qBit 重启后 cookie 被弃），重新登录再试一次
	if err == nil && status == http.StatusForbidden {
		if relog := q.login(); relog == nil {
			body, status, err = q.doAddTorrent(ctx, torrentURL, savePath)
		}
	}
	if err != nil {
		return "", fmt.Errorf("添加种子失败: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("添加种子失败 status=%d: %s", status, body)
	}
	if strings.Contains(body, "Fails") {
		return "", fmt.Errorf("qBit 拒绝添加种子: %s", body)
	}
	return "", nil
}

// doAddTorrent 单次请求（不含 relogin 重试），返回 (body, status, err)
func (q *QBittorrent) doAddTorrent(ctx context.Context, torrentURL, savePath string) (string, int, error) {
	form := url.Values{}
	form.Set("urls", torrentURL)
	if savePath != "" {
		form.Set("savepath", savePath)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", q.baseURL+"/api/v2/torrents/add",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", q.baseURL)
	resp, err := q.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode, nil
}

func (q *QBittorrent) PauseTorrent(ctx context.Context, torrentID string) error {
	data := url.Values{}
	data.Set("hashes", torrentID)

	req, err := http.NewRequestWithContext(ctx, "POST", q.baseURL+"/api/v2/torrents/pause", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("暂停种子失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("暂停种子失败: %s", string(body))
	}

	return nil
}

func (q *QBittorrent) ResumeTorrent(ctx context.Context, torrentID string) error {
	data := url.Values{}
	data.Set("hashes", torrentID)

	req, err := http.NewRequestWithContext(ctx, "POST", q.baseURL+"/api/v2/torrents/resume", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("恢复种子失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("恢复种子失败: %s", string(body))
	}

	return nil
}

func (q *QBittorrent) RemoveTorrent(ctx context.Context, torrentID string, removeFiles bool) error {
	data := url.Values{}
	data.Set("hashes", torrentID)
	data.Set("deleteFiles", fmt.Sprintf("%t", removeFiles))

	req, err := http.NewRequestWithContext(ctx, "POST", q.baseURL+"/api/v2/torrents/delete", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("删除种子失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除种子失败: %s", string(body))
	}

	return nil
}

func (q *QBittorrent) GetTorrentInfo(ctx context.Context, torrentID string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", q.baseURL+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取种子信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取种子信息失败: %s", string(body))
	}

	var torrents []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 如果指定了 torrentID，则查找特定的种子
	if torrentID != "" {
		for _, torrent := range torrents {
			if hash, ok := torrent["hash"].(string); ok && hash == torrentID {
				return torrent, nil
			}
		}
		return nil, fmt.Errorf("未找到种子: %s", torrentID)
	}

	// 返回所有种子信息
	result := make(map[string]interface{})
	for i, torrent := range torrents {
		result[fmt.Sprintf("torrent_%d", i)] = torrent
	}
	return result, nil
}

func (q *QBittorrent) Name() string {
	return "qBittorrent"
}

func (q *QBittorrent) ListTorrents(ctx context.Context) ([]downloader.TorrentSnapshot, error) {
	if err := q.login(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.baseURL+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", q.baseURL)
	resp, err := q.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qBittorrent 列表返回 status=%d: %s", resp.StatusCode, body)
	}
	var raw []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]downloader.TorrentSnapshot, 0, len(raw))
	for _, item := range raw {
		out = append(out, downloader.TorrentSnapshot{
			ID:             stringField(item, "hash"),
			Name:           stringField(item, "name"),
			State:          stringField(item, "state"),
			HasMetadata:    boolField(item, "has_metadata", stringField(item, "state") != "metaDL"),
			Size:           int64Field(item, "size"),
			Downloaded:     int64Field(item, "downloaded"),
			DownloadSpeed:  int64Field(item, "dlspeed"),
			Progress:       floatField(item, "progress"),
			Availability:   floatFieldDefault(item, "availability", -1),
			ConnectedSeeds: int(int64Field(item, "num_seeds")),
			ETASeconds:     int(int64Field(item, "eta")),
			ContentPath:    stringField(item, "content_path"),
		})
	}
	return out, nil
}

func (q *QBittorrent) GetTotalWasted(ctx context.Context, torrentID string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		q.baseURL+"/api/v2/torrents/properties?hash="+url.QueryEscape(strings.ToLower(torrentID)), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Referer", q.baseURL)
	resp, err := q.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("qBittorrent 属性返回 status=%d: %s", resp.StatusCode, body)
	}
	var data struct {
		TotalWasted float64 `json:"total_wasted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}
	if data.TotalWasted < 0 {
		return 0, nil
	}
	return int64(data.TotalWasted), nil
}

func (q *QBittorrent) SetForceStart(ctx context.Context, torrentID string, enabled bool) error {
	values := url.Values{
		"hashes": {strings.ToLower(torrentID)},
		"value":  {fmt.Sprintf("%t", enabled)},
	}
	return q.postForm(ctx, "/api/v2/torrents/setForceStart", values)
}

func (q *QBittorrent) SetPolicy(ctx context.Context, policy downloader.TorrentPolicy) error {
	maxDownloads := policy.MaxActiveDownloads
	if maxDownloads <= 0 {
		maxDownloads = 3
	}
	maxTorrents := policy.MaxActiveTorrents
	if maxTorrents <= 0 {
		maxTorrents = maxDownloads + 2
	}
	prefs := map[string]interface{}{
		"dont_count_slow_torrents":       true,
		"incomplete_files_ext":           true,
		"slow_torrent_dl_rate_threshold": 10,
		"slow_torrent_inactive_timer":    60,
		"max_active_downloads":           maxDownloads,
		"max_active_torrents":            maxTorrents,
		"dl_limit":                       policy.DownloadRate,
		"up_limit":                       policy.UploadRate,
	}
	raw, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	return q.postForm(ctx, "/api/v2/app/setPreferences", url.Values{"json": {string(raw)}})
}

// SetHTTPProxy is intentionally a no-op for the legacy external provider. Its
// proxy belongs to qBittorrent itself rather than AniDog's process.
func (q *QBittorrent) SetHTTPProxy(context.Context, string) error { return nil }

func (q *QBittorrent) Health(ctx context.Context) downloader.EngineHealth {
	health := downloader.EngineHealth{Name: q.Name()}
	list, err := q.ListTorrents(ctx)
	if err != nil {
		return health
	}
	health.Online = true
	health.TorrentCount = len(list)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.baseURL+"/api/v2/app/version", nil)
	if err == nil {
		if resp, requestErr := q.client.Do(req); requestErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
				health.Version = strings.TrimSpace(string(body))
			}
		}
	}
	return health
}

func (q *QBittorrent) Close() error { return nil }

func (q *QBittorrent) postForm(ctx context.Context, path string, values url.Values) error {
	if err := q.login(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.baseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", q.baseURL)
	resp, err := q.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qBittorrent %s 返回 status=%d: %s", path, resp.StatusCode, body)
	}
	return nil
}

func stringField(item map[string]interface{}, key string) string {
	value, _ := item[key].(string)
	return value
}

func boolField(item map[string]interface{}, key string, fallback bool) bool {
	value, ok := item[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func int64Field(item map[string]interface{}, key string) int64 {
	switch value := item[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}

func floatField(item map[string]interface{}, key string) float64 {
	return floatFieldDefault(item, key, 0)
}

func floatFieldDefault(item map[string]interface{}, key string, fallback float64) float64 {
	value, ok := item[key].(float64)
	if !ok {
		return fallback
	}
	return value
}

var _ downloader.TorrentEngine = (*QBittorrent)(nil)
