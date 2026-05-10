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

func NewProvider(cfg *config.Config) (downloader.Downloader, error) {
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
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("登录失败 status=%d: %s", resp.StatusCode, string(body))
	}
	// qBit 登录成功返回 "Ok."，失败返回 "Fails."
	if !strings.Contains(string(body), "Ok") {
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
