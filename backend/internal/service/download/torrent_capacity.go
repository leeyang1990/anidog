package download

import (
	"context"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
	"go.uber.org/zap"
)

// Magnets may reveal a much larger payload after admission. Recheck the real
// size before continuing, retaining the partial files and a visible pause reason.
func (s *QBitSyncer) holdOversizedTorrent(ctx context.Context, dl *model.Download, info map[string]interface{}, updates map[string]interface{}) bool {
	service := s.raceService
	if service == nil || service.cfg == nil || (dl.Status != model.DownloadStatusPending && dl.Status != model.DownloadStatusDownloading) {
		return false
	}
	value, ok := info["size"].(float64)
	progress, _ := info["progress"].(float64)
	if !ok || value <= 0 || progress >= 1 {
		return false
	}
	actual := int64(value)
	if dl.TotalBytes != nil && actual <= *dl.TotalBytes && !strings.HasPrefix(dl.QualityNote, "空间保护") {
		return false
	}
	fresh := *dl
	fresh.TotalBytes = &actual
	if downloaded, ok := info["downloaded"].(float64); ok {
		n := int64(downloaded)
		fresh.DownloadedBytes = &n
	}
	service.capacityMu.Lock()
	err := service.ensureTaskCapacity(ctx, capacityTaskFromDownload(&fresh, true), dl.ID)
	// Publish the larger reservation even if this candidate must pause.
	if dbErr := service.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", dl.ID).UpdateColumn("total_bytes", actual).Error; err == nil {
		err = dbErr
	}
	service.capacityMu.Unlock()
	if err == nil {
		return false
	}
	updates["quality_note"] = "空间保护：" + err.Error()
	executor := service.executors[model.DownloadTypeTorrent]
	if executor == nil {
		return false
	}
	if pauseErr := executor.Pause(executorTaskID(dl)); pauseErr != nil {
		zap.L().Warn("空间不足时暂停种子失败，下轮重试", zap.Uint("id", dl.ID), zap.Error(pauseErr))
		return false
	}
	updates["status"] = model.DownloadStatusPaused
	updates["quality_note"] = "空间保护已暂停：" + err.Error() + "；释放空间后可继续"
	if err := service.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", dl.ID).Updates(updates).Error; err != nil {
		zap.L().Warn("保存空间保护暂停状态失败", zap.Error(err))
	}
	return true
}
