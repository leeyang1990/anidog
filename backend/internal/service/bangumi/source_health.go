package bangumi

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
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
		status, note, err := s.evaluate(ctx, anime)
		if err != nil {
			zap.L().Warn("读取源健康样本失败", zap.Uint("anime_id", anime.ID), zap.Error(err))
			continue
		}
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
		var samples []model.Download
		if err := s.db.WithContext(ctx).
			Select("id", "status", "failure_kind", "failure_code", "failure_stage", "last_error").
			Where("stream_rule_id = ? AND download_type = ? AND created_at > ?", rule.ID, model.DownloadTypeStream, now.Add(-24*time.Hour)).
			Where("status IN ?", []string{model.DownloadStatusCompleted, model.DownloadStatusFailed}).
			Order("updated_at DESC").Order("id DESC").Find(&samples).Error; err != nil {
			continue
		}
		completed, failed, latestCompleted := healthCounts(samples)
		total := completed + failed

		status := ""
		note := ""
		if total > 0 {
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
			// A new verified complete transfer proves that the circuit can close.
			// Old repeated failures must not suppress a repaired source for 24 hours.
			if status == SourceBroken && latestCompleted {
				status = SourceDegraded
				note += "（最近一次完整下载成功，恢复调度）"
			}
			// 防"翻牌"：之前已是 broken 的规则，至少需要 3 条最近成功才能翻 healthy。
			// 不然只要清掉失败历史 + 1 条偶发 manual 成功就能让规则被重新调度。
			if status == SourceHealthy && rule.HealthStatus != nil && *rule.HealthStatus == SourceBroken {
				if completed < 3 {
					status = SourceDegraded
					note += "（曾标 broken，需累计 ≥3 完成才能恢复 healthy）"
				}
			}
		} else if rule.HealthStatus != nil && *rule.HealthStatus != "" {
			// total=0：24h 内没有任何样本。保持原状态，不要把 broken 抹成空字符串
			// 让 Orchestrator 误以为可用。
			status = *rule.HealthStatus
			if rule.HealthNote != nil {
				note = *rule.HealthNote
			}
		}

		updates := map[string]interface{}{
			"health_status": status,
			"health_note":   note,
		}
		// health_at 表示进入当前状态或最近一次 broken 恢复探测的时间。
		// 状态未变化时不能每 3 分钟刷新，否则 broken 熔断永远无法 half-open。
		oldStatus := ""
		if rule.HealthStatus != nil {
			oldStatus = *rule.HealthStatus
		}
		if rule.HealthAt == nil || oldStatus != status {
			updates["health_at"] = &now
		}
		s.db.WithContext(ctx).Model(rule).Updates(updates)
	}
}

// evaluate 根据下载历史计算某部番的健康状态
func (s *SourceHealthService) evaluate(ctx context.Context, anime *model.Anime) (string, string, error) {
	if anime.StreamDetailURL == nil {
		return "", "", nil
	}

	// 查询该 anime + 当前源偏好的下载记录
	var downloads []model.Download
	q := s.db.WithContext(ctx).
		Where("anime_id = ?", anime.ID).
		Where("stream_detail_url = ?", *anime.StreamDetailURL)
	if anime.StreamRuleID != nil {
		q = q.Where("stream_rule_id = ?", *anime.StreamRuleID)
	}
	if anime.StreamRoadName != nil {
		q = q.Where("stream_road_name = ?", *anime.StreamRoadName)
	}
	if err := q.Where("download_type = ?", model.DownloadTypeStream).Order("updated_at DESC").Order("id DESC").Limit(20).Find(&downloads).Error; err != nil {
		return "", "", err
	}

	if len(downloads) == 0 {
		return "", "", nil
	}

	completed, failed, _ := healthCounts(downloads)

	total := completed + failed
	if total == 0 {
		// 还在下载中，无法判断
		return "", "", nil
	}

	failRate := float64(failed) / float64(total)
	note := fmt.Sprintf("最近 %d 条：%d 完成 / %d 失败", total, completed, failed)

	if failed == 0 {
		return SourceHealthy, note, nil
	}
	if failRate >= 0.8 {
		return SourceBroken, note + "，建议切换源", nil
	}
	if failRate >= 0.3 {
		return SourceDegraded, note, nil
	}
	return SourceHealthy, note, nil
}

// healthCounts consumes only completed transfers and failures that establish
// source availability. Callers order newest first; ignored local/quality events
// cannot reopen or prolong a source circuit.
func healthCounts(rows []model.Download) (completed, failed int, latestCompleted bool) {
	seen := false
	for _, row := range rows {
		success := row.Status == model.DownloadStatusCompleted
		outage := row.Status == model.DownloadStatusFailed && failure.FromRecord(row).AvailabilityFailure
		if !success && !outage {
			continue
		}
		if !seen {
			latestCompleted, seen = success, true
		}
		if success {
			completed++
		} else {
			failed++
		}
	}
	return
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
