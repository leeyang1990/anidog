package scheduler

import (
	"context"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MaintenanceJob 清理已经失去运行价值的竞速候选记录。
// completed/failed 记录保留用于追踪问题，只删除过期 superseded 行。
type MaintenanceJob struct {
	db        *gorm.DB
	retention time.Duration
}

func NewMaintenanceJob(db *gorm.DB, retention time.Duration) *MaintenanceJob {
	if retention <= 0 {
		retention = 90 * 24 * time.Hour
	}
	return &MaintenanceJob{db: db, retention: retention}
}

func (j *MaintenanceJob) Name() string { return "maintenance" }

func (j *MaintenanceJob) Run(ctx context.Context) {
	if j == nil || j.db == nil {
		return
	}
	cutoff := time.Now().Add(-j.retention)
	result := j.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", model.DownloadStatusSuperseded, cutoff).
		Delete(&model.Download{})
	if result.Error != nil {
		zap.L().Warn("清理过期竞速记录失败", zap.Error(result.Error))
		return
	}
	if result.RowsAffected > 0 {
		zap.L().Info("已清理过期竞速记录", zap.Int64("rows", result.RowsAffected))
	}
}
