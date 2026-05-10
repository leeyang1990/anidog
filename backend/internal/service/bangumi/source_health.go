package bangumi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

// SourceHealthStatus 源健康状态枚举
const (
	SourceHealthy  = "healthy"
	SourceDegraded = "degraded"
	SourceBroken   = "broken"
)

// SourceHealthService 定期检查每部追番的下载源健康状态。
// 不阻塞下载主流程，仅基于 download 表的历史记录判断。
type SourceHealthService struct {
	db *gorm.DB
}

func NewSourceHealthService(db *gorm.DB) *SourceHealthService {
	return &SourceHealthService{db: db}
}

// CheckAllSubscribed 扫描所有已追番，更新 source_health_status
func (s *SourceHealthService) CheckAllSubscribed(ctx context.Context) {
	var animes []model.Anime
	if err := s.db.WithContext(ctx).
		Where("is_subscribed = ? AND stream_detail_url IS NOT NULL", true).
		Find(&animes).Error; err != nil {
		zap.L().Error("查询追番失败", zap.Error(err))
		return
	}

	now := time.Now()
	for i := range animes {
		anime := &animes[i]
		status, note := s.evaluate(ctx, anime)
		updates := map[string]interface{}{
			"source_health_status": status,
			"source_health_note":   note,
			"source_health_at":     &now,
		}
		if err := s.db.WithContext(ctx).Model(anime).Updates(updates).Error; err != nil {
			zap.L().Debug("更新健康状态失败", zap.Uint("id", anime.ID), zap.Error(err))
		}
	}

	// 规则级聚合
	s.updateRuleHealth(ctx)

	zap.L().Info("源健康检测完成", zap.Int("count", len(animes)))
}

// updateRuleHealth 聚合每个规则的下载成败率，标记规则健康度
func (s *SourceHealthService) updateRuleHealth(ctx context.Context) {
	var rules []model.StreamRule
	if err := s.db.WithContext(ctx).Find(&rules).Error; err != nil {
		return
	}

	now := time.Now()
	for i := range rules {
		rule := &rules[i]
		var total int64
		var failed int64
		// 取最近 24 小时该规则的所有 stream 下载
		q := s.db.WithContext(ctx).Model(&model.Download{}).
			Where("stream_rule_id = ?", rule.ID).
			Where("download_type = ?", "stream").
			Where("created_at > ?", now.Add(-24*time.Hour))
		q.Count(&total)
		q.Where("status = ?", model.DownloadStatusFailed).Count(&failed)

		status := ""
		note := ""
		if total > 0 {
			completed := total - failed
			rate := float64(failed) / float64(total)
			note = fmt.Sprintf("24h 内：%d 完成 / %d 失败", completed, failed)
			switch {
			case rate >= 0.8:
				status = SourceBroken
			case rate >= 0.3:
				status = SourceDegraded
			default:
				status = SourceHealthy
			}
		}

		s.db.WithContext(ctx).Model(rule).Updates(map[string]interface{}{
			"health_status": status,
			"health_note":   note,
			"health_at":     &now,
		})
	}
}

// evaluate 根据下载历史计算某部番的健康状态
func (s *SourceHealthService) evaluate(ctx context.Context, anime *model.Anime) (string, string) {
	if anime.StreamDetailURL == nil {
		return "", ""
	}

	// 查询该 anime + 当前源偏好的下载记录
	var downloads []model.Download
	q := s.db.WithContext(ctx).
		Where("anime_id = ?", anime.ID).
		Where("stream_detail_url = ?", *anime.StreamDetailURL)
	if anime.StreamRoadName != nil {
		q = q.Where("stream_road_name = ?", *anime.StreamRoadName)
	}
	q.Order("created_at DESC").Limit(20).Find(&downloads)

	if len(downloads) == 0 {
		return "", ""
	}

	completed, failed, disguised := 0, 0, 0
	for _, d := range downloads {
		switch d.Status {
		case model.DownloadStatusCompleted:
			completed++
		case model.DownloadStatusFailed:
			failed++
			// TODO: 如果 download 表里记了 error_message，可以检测"伪装流"关键字
		}
	}
	_ = disguised

	total := completed + failed
	if total == 0 {
		// 还在下载中，无法判断
		return "", ""
	}

	failRate := float64(failed) / float64(total)
	note := fmt.Sprintf("最近 %d 条：%d 完成 / %d 失败", total, completed, failed)

	if failed == 0 {
		return SourceHealthy, note
	}
	if failRate >= 0.8 {
		return SourceBroken, note + "，建议切换源"
	}
	if failRate >= 0.3 {
		return SourceDegraded, note
	}
	return SourceHealthy, note
}

// ClearHealthStatus 当用户主动切换源时调用，清空状态等待新一轮评估
func (s *SourceHealthService) ClearHealthStatus(ctx context.Context, animeID uint) {
	s.db.WithContext(ctx).Model(&model.Anime{}).
		Where("id = ?", animeID).
		Updates(map[string]interface{}{
			"source_health_status": "",
			"source_health_note":   "",
			"source_health_at":     nil,
		})
}

var _ = strings.Contains // keep import for future error_message parsing
