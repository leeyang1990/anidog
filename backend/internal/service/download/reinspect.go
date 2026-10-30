package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
)

func (s *Service) StartRetainedInspection(ctx context.Context, id uint) (*model.Download, error) {
	row, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.Status != model.DownloadStatusFailed || row.FilePath == nil || s.cfg == nil {
		return nil, fmt.Errorf("任务没有可复核的保留成片")
	}
	if err := checkRetainedPath(*row.FilePath, s.currentMediaRoot(ctx)); err != nil {
		return nil, fmt.Errorf("保留成片不可读取")
	}
	if _, loaded := s.inspectionRequests.LoadOrStore(id, struct{}{}); loaded {
		return nil, fmt.Errorf("复核已经排队或执行中")
	}
	go func() {
		defer s.inspectionRequests.Delete(id)
		if _, err := s.InspectRetainedMedia(context.Background(), id); err != nil {
			zap.L().Warn("保留成片复核未完成", zap.Uint("download_id", id), zap.Error(err))
		}
	}()
	row.InspectionActive = true
	return row, nil
}

// InspectRetainedMedia updates inspection facts for a failed candidate without
// fetching a source, changing a file or granting archive/identity approval.
// It also provides a safe migration path for old rejected-but-retained files.
func (s *Service) InspectRetainedMedia(ctx context.Context, id uint) (*model.Download, error) {
	var row model.Download
	if err := s.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	if row.Status != model.DownloadStatusFailed || row.FilePath == nil || s.cfg == nil {
		return nil, fmt.Errorf("只有下载目录内保留的失败成片可以复核")
	}
	if err := checkRetainedPath(*row.FilePath, s.currentMediaRoot(ctx)); err != nil {
		return nil, fmt.Errorf("保留成片不可读取")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	var previous mediacheck.SubtitleEvidence
	if json.Unmarshal([]byte(row.SubtitleEvidence), &previous) == nil && previous.InspectorVersion == mediacheck.SubtitleInspectorVersion && previous.Language == "" && !previous.Retryable {
		ctx = context.WithValue(ctx, refreshUnknownEvidence{}, true)
	}
	run := newValidationRun(cancel)
	if _, loaded := s.validationCancels.LoadOrStore(id, run); loaded {
		return nil, fmt.Errorf("此成片已有检查正在运行")
	}
	defer s.finishValidationRun(id, run)
	if s.validationSlots != nil {
		select {
		case s.validationSlots <- struct{}{}:
			defer func() { <-s.validationSlots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	err := s.validateMedia(ctx, &row, valueOrZero(row.EpisodeNumber), *row.FilePath, true)
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil, ctx.Err()
	}
	updates := map[string]interface{}{}
	if err == nil {
		updates = map[string]interface{}{"quality_note": "成片画质与字幕复核通过；尚未执行身份复核和归档，请显式重试", "failure_code": "", "failure_stage": "", "failure_kind": model.FailureKindReview, "next_retry_at": nil, "last_error": "成片复核通过，等待显式确认原候选身份和归档"}
	} else {
		updates, _, _ = failureFields(err, row.RetryCount)
		updates["last_error"] = err.Error()
		updates["quality_note"] = err.Error()
	}
	q := s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ? AND status = ? AND file_path = ? AND video_hash = ?", id, model.DownloadStatusFailed, *row.FilePath, row.VideoHash).Updates(updates)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected != 1 {
		return nil, fmt.Errorf("复核期间任务状态已变化，未覆盖新状态")
	}
	// Reload into a fresh value: SQL NULL must clear a previous retry deadline.
	row = model.Download{}
	if err = s.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	row.ResultClass = failure.ResultClass(row)
	return &row, nil
}

func checkRetainedPath(path, root string) error {
	stat, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !stat.Mode().IsRegular() || stat.Size() == 0 {
		return fmt.Errorf("保留成片不是完整的普通文件")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if !pathWithin(resolved, resolvedRoot) {
		return fmt.Errorf("保留成片路径越过媒体目录")
	}
	return nil
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
