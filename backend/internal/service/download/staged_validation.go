package download

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
	"go.uber.org/zap"
)

// retainStreamResult records the payload before any slow local acceptance. A
// restart, expiring URL, or inspection outage must not force another transfer.
func (s *Service) retainStreamResult(ctx context.Context, id uint, result *Result) error {
	if result == nil || result.FilePath == "" || result.FinalPath == "" {
		return nil
	}
	stat, err := os.Stat(existingCompletionPath(result.FilePath, result.FinalPath))
	if err != nil {
		return failure.Wrap(failure.Validation, failure.StorageUnavailable, err)
	}
	updates := map[string]interface{}{"file_path": result.FilePath, "validation_final_path": result.FinalPath, "validation_pending": true,
		"downloaded_bytes": stat.Size(), "total_bytes": stat.Size(), "progress": 100.0, "download_speed": 0}
	q := s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ? AND status IN ?", id, []string{model.DownloadStatusPending, model.DownloadStatusDownloading, model.DownloadStatusFailed}).Updates(updates)
	if q.Error != nil {
		return failure.Wrap(failure.Validation, failure.StateUnavailable, q.Error)
	}
	if q.RowsAffected != 1 {
		return failure.Wrap(failure.Validation, failure.Canceled, fmt.Errorf("暂存成片所属任务已结束"))
	}
	return nil
}

func (s *Service) retainedValidationResult(ctx context.Context, dl *model.Download) (*Result, error) {
	if !dl.ValidationPending {
		return nil, nil
	}
	if dl.FilePath == nil || *dl.FilePath == "" || dl.ValidationFinalPath == "" {
		return nil, failure.Wrap(failure.Validation, failure.StateUnavailable, fmt.Errorf("暂存成片缺少验收路径"))
	}
	root := ""
	if dl.DownloadType == model.DownloadTypeTorrent && s.cfg != nil {
		root = s.currentMediaRoot(ctx)
	}
	if root == "" && dl.SavePath != nil {
		root = *dl.SavePath
	}
	if root == "" && s.cfg != nil {
		root = s.currentMediaRoot(ctx)
	}
	if root == "" || !pathWithin(*dl.FilePath, root) || !pathWithin(dl.ValidationFinalPath, root) || dl.DownloadType == model.DownloadTypeStream && filepath.Dir(*dl.FilePath) != filepath.Dir(dl.ValidationFinalPath) {
		return nil, failure.Wrap(failure.Validation, failure.StorageUnavailable, fmt.Errorf("暂存成片路径超出下载目录"))
	}
	path := existingCompletionPath(*dl.FilePath, dl.ValidationFinalPath)
	stat, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, failure.Wrap(failure.Validation, failure.StorageUnavailable, err)
	}
	if !stat.Mode().IsRegular() || stat.Size() == 0 {
		return nil, failure.Wrap(failure.Validation, failure.InvalidMedia, fmt.Errorf("暂存成片不是完整的普通文件"))
	}
	if err := checkRetainedPath(path, root); err != nil {
		return nil, failure.Wrap(failure.Validation, failure.StorageUnavailable, err)
	}
	return &Result{FilePath: *dl.FilePath, FinalPath: dl.ValidationFinalPath}, nil
}

// HasRetainedMedia is the asset availability boundary for source planning.
// Cooling a source cannot invalidate bytes already awaiting local acceptance.
// Missing files permit a new transfer; unsafe/unreadable paths fail closed.
func (s *Service) HasRetainedMedia(ctx context.Context, dl *model.Download) (bool, error) {
	result, err := s.retainedValidationResult(ctx, dl)
	return result != nil, err
}

type validationRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func newValidationRun(cancel context.CancelFunc) *validationRun {
	return &validationRun{cancel: cancel, done: make(chan struct{})}
}

func (s *Service) finishValidationRun(id uint, run *validationRun) {
	run.cancel()
	s.validationCancels.CompareAndDelete(id, run)
	close(run.done)
}

// Resume waits for a paused worker's lease to end before claiming another. A
// worker may have read the paused state just before the resume request arrived.
func (s *Service) resumeValidation(id uint, staging, final string) error {
	if value, loaded := s.validationCancels.Load(id); loaded {
		run := value.(*validationRun)
		go func() {
			select {
			case <-run.done:
				var row model.Download
				if s.db.First(&row, id).Error == nil && row.Status == model.DownloadStatusDownloading && row.ValidationPending {
					if err := s.QueueEpisodeCompletion(context.Background(), id, staging, final); err != nil {
						zap.L().Warn("恢复成片验收失败", zap.Uint("download_id", id), zap.Error(err))
					}
				}
			case <-time.After(8 * time.Minute):
			}
		}()
		return nil
	}
	return s.QueueEpisodeCompletion(context.Background(), id, staging, final)
}

func (s *Service) cancelValidation(id uint) {
	if cancel, ok := s.validationCancels.Load(id); ok {
		cancel.(*validationRun).cancel()
	}
}

func (s *Service) finishStreamResult(ctx context.Context, id uint, task *Task, result *Result) {
	ctx, cancel := context.WithCancel(ctx)
	run := newValidationRun(cancel)
	if _, loaded := s.validationCancels.LoadOrStore(id, run); loaded {
		cancel()
		return
	}
	defer s.finishValidationRun(id, run)
	s.finishMediaValidation(ctx, id, task, result)
}

// QueueEpisodeCompletion keeps slow local OCR out of BT's global health patrol.
// Claim before spawning, retain transfer completion before inspection, and share
// the same worker slots and cancellation lease as stream acceptance.
func (s *Service) QueueEpisodeCompletion(ctx context.Context, id uint, staging, final string) error {
	var row model.Download
	if err := s.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return err
	}
	if row.Status != model.DownloadStatusDownloading && row.Status != model.DownloadStatusPending {
		return nil
	}
	jobCtx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	run := newValidationRun(cancel)
	if _, loaded := s.validationCancels.LoadOrStore(id, run); loaded {
		cancel()
		return nil
	}
	result := &Result{FilePath: staging, FinalPath: final}
	if err := s.retainStreamResult(ctx, id, result); err != nil {
		s.finishValidationRun(id, run)
		return err
	}
	go func() {
		defer s.finishValidationRun(id, run)
		s.finishMediaValidation(jobCtx, id, validationTask(&row), result)
	}()
	return nil
}

func (s *Service) finishMediaValidation(ctx context.Context, id uint, task *Task, result *Result) {
	if s.validationSlots != nil {
		select {
		case s.validationSlots <- struct{}{}:
			defer func() { <-s.validationSlots }()
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fields, _, _ := failureFields(failure.Wrap(failure.Validation, failure.InspectionFailed, ctx.Err()), task.RetryCount)
				s.updateStatus(id, model.DownloadStatusFailed, fields)
			}
			return
		}
	}
	var row model.Download
	if err := s.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return
	}
	if row.Status != model.DownloadStatusDownloading && row.Status != model.DownloadStatusPending {
		return
	}
	path := existingCompletionPath(result.FilePath, result.FinalPath)
	if err := s.validateStreamDurationConsistency(ctx, task, path); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		err = failure.Wrap(failure.Validation, failure.InvalidMedia, fmt.Errorf("流媒体成片时长检查失败：%w", err))
		row.FilePath = &path
		if failure.Describe(err).Kind == model.FailureKindRejected {
			err = candidateMediaError{err}
		}
		s.rejectMediaCandidate(ctx, &row, err)
		return
	}
	if s.CompleteEpisodeRace(ctx, id, result.FilePath, result.FinalPath) && s.hub != nil {
		s.hub.BroadcastDownloadComplete(task.RuntimeID, task.Name)
	}
}

// ResumeMediaValidation is called before searching missing episodes. It claims
// retained payloads with a CAS and spends the same bounded attempt budget, but
// never resets it, fetches a playback page, or creates another download row.
func (s *Service) ResumeMediaValidation(ctx context.Context, animeID uint) error {
	var rows []model.Download
	err := s.db.WithContext(ctx).Where("anime_id = ? AND validation_pending = ? AND status = ? AND retry_count < ?", animeID, true, model.DownloadStatusFailed, 3).
		Where("failure_code <> ? AND failure_kind NOT IN ?", string(failure.Canceled), []string{model.FailureKindRejected, model.FailureKindReview}).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", time.Now()).
		Where("episode_number IS NOT NULL").
		Where(`NOT EXISTS (SELECT 1 FROM download newer WHERE newer.anime_id = download.anime_id AND newer.episode_number = download.episode_number AND newer.id > download.id AND newer.validation_pending = ? AND newer.status = ?)`, true, model.DownloadStatusFailed).
		Where(`NOT EXISTS (SELECT 1 FROM download active WHERE active.anime_id = download.anime_id AND active.episode_number = download.episode_number AND active.id <> download.id AND (active.status IN ? OR (active.status = ? AND active.media_missing = ?)))`, []string{model.DownloadStatusPending, model.DownloadStatusDownloading}, model.DownloadStatusCompleted, false).
		Order("id DESC").Limit(24).Find(&rows).Error
	if err != nil {
		return err
	}
	for i := range rows {
		row := &rows[i]
		retained, err := s.retainedValidationResult(ctx, row)
		if err != nil {
			return err
		}
		if retained == nil {
			if err := s.db.WithContext(ctx).Model(row).Updates(map[string]interface{}{"validation_pending": false, "validation_final_path": "", "file_path": nil}).Error; err != nil {
				return err
			}
			continue
		}
		if !s.HasExecutor(row.DownloadType) {
			continue
		}
		updates := map[string]interface{}{"status": model.DownloadStatusPending, "retry_count": row.RetryCount + 1, "failure_kind": "", "failure_code": "", "failure_stage": "", "last_error": "", "next_retry_at": nil}
		q := s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ? AND status = ? AND retry_count = ? AND validation_pending = ?", row.ID, model.DownloadStatusFailed, row.RetryCount, true).Updates(updates)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			continue
		}
		task := validationTask(row)
		task.RetryCount = row.RetryCount + 1
		go s.execute(row.ID, row.TorrentID, task)
		zap.L().Info("恢复暂存成片本地验收", zap.Uint("download_id", row.ID))
	}
	return nil
}

func validationTask(row *model.Download) *Task {
	task := capacityTaskFromDownload(row, false)
	task.Name, task.AnimeName, task.SourceTitle = row.Name, row.Name, row.SourceTitle
	task.URL, task.Source = row.URL, row.Source
	task.AnimeID, task.EpisodeNumber = row.AnimeID, row.EpisodeNumber
	task.Purpose, task.UpgradeTarget, task.UpgradeReason = row.Purpose, row.UpgradeTarget, row.UpgradeReason
	task.StreamRuleID = row.StreamRuleID
	if row.StreamDetailURL != nil {
		task.StreamDetailURL = *row.StreamDetailURL
	}
	if row.StreamRoadName != nil {
		task.StreamRoadName = *row.StreamRoadName
	}
	return task
}
