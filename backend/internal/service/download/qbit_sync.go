package download

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
)

// QBitSyncer 从 qBit 同步 BT 下载的真实进度/大小到 DB。
// 设计为 scheduler.Job，可定时调用。
type QBitSyncer struct {
	db      *gorm.DB
	baseURL string
	user    string
	pass    string
	client  *http.Client
}

func NewQBitSyncer(db *gorm.DB, cfg *config.Config) *QBitSyncer {
	jar, _ := cookiejar.New(nil)
	return &QBitSyncer{
		db:      db,
		baseURL: strings.TrimSuffix(cfg.DownloaderHost, "/"),
		user:    cfg.DownloaderUsername,
		pass:    cfg.DownloaderPassword,
		client:  &http.Client{Jar: jar, Timeout: 15 * time.Second},
	}
}

func (s *QBitSyncer) Name() string { return "qbit_sync" }

// Run 实现 scheduler.Job 接口。
func (s *QBitSyncer) Run(ctx context.Context) {
	if err := s.Sync(ctx); err != nil {
		zap.L().Warn("qBit 同步失败", zap.Error(err))
	}
}

// Sync 拉取 qBit 所有种子信息，按 info_hash 更新对应 Download 记录。
func (s *QBitSyncer) Sync(ctx context.Context) error {
	if err := s.ensureLogin(ctx); err != nil {
		return err
	}

	torrents, err := s.listTorrents(ctx)
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		return nil
	}

	// 按 info_hash 做映射
	byHash := make(map[string]map[string]interface{}, len(torrents))
	for _, t := range torrents {
		if h, ok := t["hash"].(string); ok && h != "" {
			byHash[strings.ToUpper(h)] = t
		}
	}

	// 查 DB 里所有有 info_hash 的 BT 下载
	var downloads []model.Download
	if err := s.db.WithContext(ctx).
		Where("download_type = ? AND info_hash IS NOT NULL", model.DownloadTypeTorrent).
		Find(&downloads).Error; err != nil {
		return err
	}

	updated := 0
	for _, dl := range downloads {
		if dl.InfoHash == nil {
			continue
		}
		qt, ok := byHash[strings.ToUpper(*dl.InfoHash)]
		if !ok {
			continue
		}
		updates := map[string]interface{}{}
		if v, ok := qt["size"].(float64); ok && v > 0 {
			size := int64(v)
			updates["total_bytes"] = size
		}
		if v, ok := qt["downloaded"].(float64); ok {
			updates["downloaded_bytes"] = int64(v)
		}
		if v, ok := qt["progress"].(float64); ok {
			updates["progress"] = v * 100.0
		}
		if v, ok := qt["dlspeed"].(float64); ok {
			updates["download_speed"] = int64(v)
		}
		if v, ok := qt["eta"].(float64); ok && v > 0 && v < 8640000 {
			eta := int(v)
			updates["eta"] = eta
		}
		// 状态映射
		if state, ok := qt["state"].(string); ok {
			newStatus := mapQBitState(state)
			if newStatus != "" && newStatus != dl.Status {
				updates["status"] = newStatus
				if newStatus == model.DownloadStatusCompleted && dl.CompletedAt == nil {
					now := time.Now()
					updates["completed_at"] = &now
				}
			}
		}

		if len(updates) > 0 {
			if err := s.db.Model(&model.Download{}).
				Where("id = ?", dl.ID).
				Updates(updates).Error; err != nil {
				zap.L().Warn("更新下载进度失败",
					zap.Uint("id", dl.ID), zap.Error(err))
				continue
			}
			updated++
		}
	}

	if updated > 0 {
		zap.L().Info("qBit 同步完成",
			zap.Int("matched", updated),
			zap.Int("qbit_total", len(torrents)),
			zap.Int("db_total", len(downloads)))
	}
	return nil
}

// mapQBitState 把 qBit 状态映射到我们的 status
func mapQBitState(qs string) string {
	switch qs {
	case "uploading", "stalledUP", "queuedUP", "forcedUP", "checkingUP", "pausedUP":
		return model.DownloadStatusCompleted
	case "downloading", "metaDL", "stalledDL", "forcedDL", "checkingDL":
		return model.DownloadStatusDownloading
	case "pausedDL":
		return model.DownloadStatusPaused
	case "queuedDL":
		return model.DownloadStatusPending
	case "error", "missingFiles":
		return model.DownloadStatusFailed
	}
	return ""
}

func (s *QBitSyncer) ensureLogin(ctx context.Context) error {
	data := url.Values{}
	data.Set("username", s.user)
	data.Set("password", s.pass)
	req, err := http.NewRequestWithContext(ctx, "POST",
		s.baseURL+"/api/v2/auth/login", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", s.baseURL)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("qBit 登录失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Ok") {
		return fmt.Errorf("qBit 登录返回 %s", string(body))
	}
	return nil
}

func (s *QBitSyncer) listTorrents(ctx context.Context) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		s.baseURL+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", s.baseURL)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 qBit 种子失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("查询 qBit 种子失败 status=%d: %s", resp.StatusCode, string(b))
	}
	var arr []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		return nil, err
	}
	return arr, nil
}
