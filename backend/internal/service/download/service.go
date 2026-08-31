package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/disk"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/notification"
	"github.com/anidog/anidog-go/internal/service/stream"
	"github.com/anidog/anidog-go/internal/ws"
)

// Service is the unified download service. All download creation and
// execution goes through this, regardless of trigger source.
type Service struct {
	db                    *gorm.DB
	cfg                   *config.Config
	hub                   *ws.Hub
	executors             map[string]Executor
	notifSvc              *notification.Service // 可空：未注入时不发通知
	rejectedStreamHandler func(context.Context, uint, int)
}

// NewService creates a new unified download service.
func NewService(db *gorm.DB, cfg *config.Config, hub *ws.Hub) *Service {
	return &Service{
		db:        db,
		cfg:       cfg,
		hub:       hub,
		executors: make(map[string]Executor),
	}
}

// RegisterExecutor registers an executor for a download type (e.g. "torrent", "stream").
func (s *Service) RegisterExecutor(downloadType string, exec Executor) {
	s.executors[downloadType] = exec
}

// HasExecutor 返回某种下载类型当前是否具备可执行后端。
func (s *Service) HasExecutor(downloadType string) bool {
	_, ok := s.executors[downloadType]
	return ok
}

// SetNotificationService 注入通知服务。
// 这是唯一的通知收口：所有下载完成事件（不管是 BT/Stream/Manual 哪条路径触发）
// 都从 updateStatus → notifyCompletion 走，避免在多处事件源各自接钩子导致漏发或重发。
func (s *Service) SetNotificationService(n *notification.Service) {
	s.notifSvc = n
}

// SetRejectedStreamHandler 注入流媒体候选质量失败后的同集恢复回调。
// 候选被拒绝后不应重复下载同一条短片或伪装流，而应立即让编排器换线路、
// 换规则，并同时尝试 BT/Mikan 等其他已启用来源。
func (s *Service) SetRejectedStreamHandler(handler func(context.Context, uint, int)) {
	s.rejectedStreamHandler = handler
}

// Create creates a Download record and starts async execution.
func (s *Service) Create(ctx context.Context, task *Task) (*model.Download, error) {
	if err := task.Validate(); err != nil {
		return nil, fmt.Errorf("invalid task: %w", err)
	}
	if err := s.ensureMediaCapacity(ctx); err != nil {
		return nil, fmt.Errorf("媒体存储容量保护: %w", err)
	}

	torrentID := generateTorrentID(task.DownloadType)
	savePath := s.resolveTaskSavePath(ctx, task)

	dl := model.Download{
		TorrentID:     torrentID,
		Name:          task.Name,
		URL:           task.URL,
		SavePath:      savePath,
		Status:        model.DownloadStatusPending,
		DownloadType:  task.DownloadType,
		StreamRuleID:  task.StreamRuleID,
		AnimeID:       task.AnimeID,
		EpisodeNumber: task.EpisodeNumber,
		Scope:         task.Scope,
		EpisodeStart:  task.EpisodeStart,
		EpisodeEnd:    task.EpisodeEnd,
		Source:        task.Source,
		RetryCount:    task.RetryCount,
	}
	if dl.Scope == "" {
		dl.Scope = model.DownloadScopeEpisode
	}
	if task.StreamRoadName != "" {
		rn := task.StreamRoadName
		dl.StreamRoadName = &rn
	}
	if task.StreamDetailURL != "" {
		du := task.StreamDetailURL
		dl.StreamDetailURL = &du
	}
	// 对 BT 任务提取 info_hash 用于后续同步 qBit 进度
	if task.DownloadType == model.DownloadTypeTorrent {
		if h := ExtractInfoHash(task.URL); h != "" {
			dl.InfoHash = &h
		}
	}

	if err := s.db.Create(&dl).Error; err != nil {
		return nil, fmt.Errorf("创建下载记录失败: %w", err)
	}

	if s.hub != nil {
		var animeID uint
		if dl.AnimeID != nil {
			animeID = *dl.AnimeID
		}
		s.hub.BroadcastDownloadProgress(torrentID, task.Name, 0, animeID)
	}

	task.RuntimeID = torrentID
	go s.execute(dl.ID, torrentID, task)

	return &dl, nil
}

// Cancel cancels a running download.
func (s *Service) Cancel(dlID uint) error {
	var dl model.Download
	if err := s.db.First(&dl, dlID).Error; err != nil {
		return fmt.Errorf("下载任务不存在")
	}

	exec := s.executors[dl.DownloadType]
	if exec == nil {
		return fmt.Errorf("无对应执行器")
	}

	if err := exec.Cancel(executorTaskID(&dl)); err != nil {
		zap.L().Warn("取消下载失败", zap.Error(err))
	}

	s.db.Model(&dl).Update("status", model.DownloadStatusFailed)
	return nil
}

// Pause pauses a download and returns the updated record.
func (s *Service) Pause(dlID uint) (*model.Download, error) {
	var dl model.Download
	if err := s.db.First(&dl, dlID).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}

	exec := s.executors[dl.DownloadType]
	if exec == nil {
		return nil, fmt.Errorf("无对应执行器")
	}

	if err := exec.Pause(executorTaskID(&dl)); err != nil {
		return nil, err
	}

	s.db.Model(&dl).Update("status", model.DownloadStatusPaused)
	dl.Status = model.DownloadStatusPaused
	return &dl, nil
}

// Resume resumes a paused download and returns the updated record.
func (s *Service) Resume(dlID uint) (*model.Download, error) {
	var dl model.Download
	if err := s.db.First(&dl, dlID).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}

	exec := s.executors[dl.DownloadType]
	if exec == nil {
		return nil, fmt.Errorf("无对应执行器")
	}

	if err := exec.Resume(executorTaskID(&dl)); err != nil {
		return nil, err
	}

	s.db.Model(&dl).Update("status", model.DownloadStatusDownloading)
	dl.Status = model.DownloadStatusDownloading
	return &dl, nil
}

// Remove removes a download and optionally its files.
func (s *Service) Remove(dlID uint, removeFiles bool) error {
	var dl model.Download
	if err := s.db.First(&dl, dlID).Error; err != nil {
		return fmt.Errorf("下载任务不存在")
	}

	exec := s.executors[dl.DownloadType]
	if exec != nil {
		if err := exec.Remove(executorTaskID(&dl), removeFiles); err != nil {
			zap.L().Warn("移除下载文件失败", zap.Error(err))
		}
	}

	return s.db.Delete(&dl).Error
}

// execute runs the download in a goroutine, updating DB state as it goes.
func (s *Service) execute(dlID uint, torrentID string, task *Task) {
	// Create 会预先写入 RuntimeID；重试/服务重启恢复的旧记录则从
	// download.torrent_id 恢复。统一在这里兜底，确保流媒体竞速候选可被
	// 精确取消，且每个 staging 文件都有稳定、唯一的名字。
	if task.RuntimeID == "" {
		task.RuntimeID = torrentID
	}
	exec, ok := s.executors[task.DownloadType]
	if !ok {
		err := fmt.Errorf("无可用的 %s 下载执行器", task.DownloadType)
		kind, delay := classifyError(err, task.RetryCount)
		extra := map[string]interface{}{
			"failure_kind": kind,
			"last_error":   err.Error(),
		}
		if delay > 0 {
			nextAt := time.Now().Add(delay)
			extra["next_retry_at"] = &nextAt
		}
		s.updateStatus(dlID, model.DownloadStatusFailed, extra)
		zap.L().Error("无对应下载执行器", zap.String("type", task.DownloadType), zap.String("failure_kind", kind))
		return
	}

	s.updateStatus(dlID, model.DownloadStatusDownloading, nil)

	progressCB := func(progress float64, downloadedBytes, totalBytes int64) {
		updates := map[string]interface{}{}
		if progress >= 0 {
			updates["progress"] = progress
		}
		if downloadedBytes > 0 {
			updates["downloaded_bytes"] = downloadedBytes
		}
		if totalBytes > 0 {
			updates["total_bytes"] = totalBytes
		}
		if len(updates) > 0 {
			s.db.Model(&model.Download{}).Where("id = ?", dlID).Updates(updates)
		}

		if s.hub != nil && progress >= 0 {
			var animeID uint
			if task.AnimeID != nil {
				animeID = *task.AnimeID
			}
			s.hub.BroadcastDownloadProgress(torrentID, task.Name, progress, animeID)
		}
	}

	ctx := context.Background()
	result, err := exec.Execute(ctx, task, progressCB)

	if err != nil {
		// 先读 retry_count，分类 + 算下次重试时间，统一写回。
		var prev model.Download
		_ = s.db.First(&prev, dlID).Error
		if prev.Status == model.DownloadStatusSuperseded {
			zap.L().Info("竞速候选已被赢家取消，忽略执行器退出错误",
				zap.Uint("id", dlID), zap.String("name", task.Name))
			return
		}
		kind, delay := classifyError(err, prev.RetryCount)
		extra := map[string]interface{}{
			"failure_kind": kind,
			"last_error":   truncateError(err),
		}
		if delay > 0 {
			nextAt := time.Now().Add(delay)
			extra["next_retry_at"] = &nextAt
		} else {
			// permanent / 已用尽重试次数 → 清空 next_retry_at
			extra["next_retry_at"] = nil
		}
		s.updateStatus(dlID, model.DownloadStatusFailed, extra)
		if kind == model.FailureKindRejected {
			s.triggerRejectedStreamRecovery(task)
		}
		if kind == model.FailureKindExhausted && task.StreamRuleID != nil {
			now := time.Now()
			note := "半开探测失败: " + truncateError(err)
			s.db.Model(&model.StreamRule{}).Where("id = ?", *task.StreamRuleID).Updates(map[string]interface{}{
				"health_status": "broken",
				"health_note":   note,
				"health_at":     &now,
			})
		}
		zap.L().Error("下载失败",
			zap.String("name", task.Name),
			zap.String("failure_kind", kind),
			zap.Duration("retry_after", delay),
			zap.Int("retry_count", prev.RetryCount),
			zap.Error(err))
		return
	}

	// TorrentExecutor 成功只表示种子已提交给 qBittorrent，并不代表文件下载完成。
	// 实际进度和 completed 只能由 QBitSyncer 根据 qBit 状态写回。
	if task.DownloadType == model.DownloadTypeTorrent {
		extra := map[string]interface{}{
			"failure_kind":  "",
			"last_error":    "",
			"next_retry_at": nil,
		}
		if result != nil && result.TorrentID != "" {
			extra["torrent_id"] = result.TorrentID
		}
		s.updateStatus(dlID, model.DownloadStatusDownloading, extra)
		zap.L().Info("种子已提交到 qBittorrent", zap.String("name", task.Name))
		return
	}

	stagingPath, finalPath := "", ""
	if result != nil {
		stagingPath, finalPath = result.FilePath, result.FinalPath
	}
	if task.DownloadType == model.DownloadTypeStream && stagingPath != "" {
		if err := s.validateStreamDurationConsistency(ctx, task, stagingPath); err != nil {
			_ = removeStagingFile(stagingPath)
			candidateErr := fmt.Errorf("流媒体候选无效：%w", err)
			s.updateStatus(dlID, model.DownloadStatusFailed, map[string]interface{}{
				"failure_kind":  model.FailureKindRejected,
				"last_error":    truncateError(candidateErr),
				"next_retry_at": nil,
			})
			s.triggerRejectedStreamRecovery(task)
			zap.L().Warn("流媒体候选未通过同番时长校验",
				zap.Uint("id", dlID), zap.String("name", task.Name), zap.Error(candidateErr))
			return
		}
	}
	if s.CompleteEpisodeRace(context.Background(), dlID, stagingPath, finalPath) && s.hub != nil {
		s.hub.BroadcastDownloadComplete(torrentID, task.Name)
	}
}

func (s *Service) triggerRejectedStreamRecovery(task *Task) {
	if task == nil || task.DownloadType != model.DownloadTypeStream ||
		task.AnimeID == nil || task.EpisodeNumber == nil ||
		*task.AnimeID == 0 || *task.EpisodeNumber <= 0 || s.rejectedStreamHandler == nil {
		return
	}
	animeID, episode := *task.AnimeID, *task.EpisodeNumber
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		s.rejectedStreamHandler(ctx, animeID, episode)
	}()
}

// CompleteEpisodeRace atomically elects the first completed candidate for an
// anime episode. Stream candidates are promoted from a unique staging path
// only after winning; losing candidates never overwrite the media file.
func (s *Service) CompleteEpisodeRace(ctx context.Context, dlID uint, stagingPath, finalPath string) bool {
	var candidate model.Download
	if err := s.db.WithContext(ctx).First(&candidate, dlID).Error; err != nil {
		_ = removeStagingFile(stagingPath)
		return false
	}
	won := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if candidate.AnimeID != nil {
			var anime model.Anime
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&anime, *candidate.AnimeID).Error; err != nil {
				return err
			}
		}
		if candidate.AnimeID != nil && candidate.EpisodeNumber != nil {
			var completedCount int64
			if err := tx.Model(&model.Download{}).
				Where("id <> ? AND anime_id = ? AND episode_number = ? AND status = ?",
					candidate.ID, *candidate.AnimeID, *candidate.EpisodeNumber, model.DownloadStatusCompleted).
				Count(&completedCount).Error; err != nil {
				return err
			}
			if completedCount > 0 {
				return tx.Model(&model.Download{}).Where("id = ?", candidate.ID).
					Updates(map[string]interface{}{
						"status":              model.DownloadStatusSuperseded,
						"download_speed":      0,
						"seeking_alternative": false,
						"quality_note":        "同集已有更早完成的候选，本任务结束竞速",
					}).Error
			}
		}
		if stagingPath != "" && finalPath != "" && stagingPath != finalPath {
			if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
				return fmt.Errorf("创建竞速赢家目录: %w", err)
			}
			if _, err := os.Stat(finalPath); err == nil {
				// 文件移动与数据库事务无法成为同一个原子操作。若上一次调用已
				// 完成 rename、但数据库提交失败，下一轮应继续补齐 DB 状态。
				if _, stagingErr := os.Stat(stagingPath); stagingErr == nil {
					return fmt.Errorf("正式媒体文件已存在，拒绝覆盖: %s", finalPath)
				} else if !os.IsNotExist(stagingErr) {
					return fmt.Errorf("检查竞速暂存文件: %w", stagingErr)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("检查正式媒体文件: %w", err)
			} else {
				if err := os.Rename(stagingPath, finalPath); err != nil {
					return fmt.Errorf("提升竞速赢家文件: %w", err)
				}
			}
		}
		now := time.Now()
		updates := map[string]interface{}{
			"status":              model.DownloadStatusCompleted,
			"progress":            100.0,
			"completed_at":        &now,
			"failure_kind":        "",
			"last_error":          "",
			"next_retry_at":       nil,
			"seeking_alternative": false,
		}
		if finalPath != "" {
			updates["file_path"] = finalPath
		}
		if err := tx.Model(&model.Download{}).Where("id = ?", candidate.ID).Updates(updates).Error; err != nil {
			return err
		}
		won = true
		return nil
	})
	if err != nil {
		zap.L().Error("剧集竞速仲裁失败", zap.Uint("id", dlID), zap.Error(err))
		// 保留完整候选，qBit/stream 下一轮同步可以重试归档。
		return false
	}
	if !won {
		_ = removeStagingFile(stagingPath)
		if exec := s.executors[candidate.DownloadType]; exec != nil {
			_ = exec.Remove(executorTaskID(&candidate), false)
		}
		return false
	}
	s.updateAnimeProgress(dlID)
	s.resolvePriorFailures(dlID)
	s.notifyCompletion(dlID)
	s.settleEpisodeRace(ctx, &candidate)
	// 隔离 BT 候选归档后，qBittorrent 仍指向旧 staging 路径。
	// 仅移除任务、不删除文件；正式媒体已原子移动到规范路径。
	if stagingPath != "" && candidate.DownloadType == model.DownloadTypeTorrent {
		if exec := s.executors[candidate.DownloadType]; exec != nil {
			if err := exec.Remove(executorTaskID(&candidate), false); err != nil {
				zap.L().Warn("移除竞速赢家的 qBit 任务失败",
					zap.Uint("id", candidate.ID), zap.Error(err))
			}
		}
		removeTorrentRaceDirectory(candidate.SavePath)
	}
	zap.L().Info("剧集竞速产生赢家",
		zap.Uint("id", dlID), zap.String("name", candidate.Name), zap.String("source", candidate.Source))
	return true
}

type seasonPackMovePlan struct {
	ep   int
	from string
	to   string
	size int64
}

// CompleteSeasonPack 把一个合集候选中的多集视频分别提升到标准媒体目录，
// 并一次性写入 AnimeEpisode。任何目标冲突或移动失败都会回滚已经移动的文件，
// 避免只归档半季却把合集标成完成。
func (s *Service) CompleteSeasonPack(ctx context.Context, dlID uint, stagingFiles map[int]string, mediaRoot string) bool {
	if len(stagingFiles) == 0 {
		return false
	}
	var candidate model.Download
	if err := s.db.WithContext(ctx).First(&candidate, dlID).Error; err != nil ||
		candidate.AnimeID == nil || candidate.Scope != model.DownloadScopeSeason {
		return false
	}
	var anime model.Anime
	if err := s.db.WithContext(ctx).First(&anime, *candidate.AnimeID).Error; err != nil {
		return false
	}

	episodes := make([]int, 0, len(stagingFiles))
	for ep := range stagingFiles {
		episodes = append(episodes, ep)
	}
	sort.Ints(episodes)
	plans := make([]seasonPackMovePlan, 0, len(episodes))
	for _, ep := range episodes {
		from := stagingFiles[ep]
		if _, err := os.Stat(from); err != nil {
			zap.L().Warn("合集归档源文件不存在", zap.Uint("id", dlID), zap.Int("episode", ep), zap.Error(err))
			return false
		}
		to := stream.BuildMediaPath(mediaRoot, &anime, ep, strings.ToLower(filepath.Ext(from)))
		if _, err := os.Stat(to); err == nil {
			zap.L().Warn("合集归档目标已存在，拒绝覆盖", zap.Uint("id", dlID), zap.Int("episode", ep), zap.String("path", to))
			return false
		} else if !os.IsNotExist(err) {
			return false
		}
		plans = append(plans, seasonPackMovePlan{ep: ep, from: from, to: to, size: fileSize(from)})
	}

	moved := make([]seasonPackMovePlan, 0, len(plans))
	for _, plan := range plans {
		if err := os.MkdirAll(filepath.Dir(plan.to), 0755); err != nil {
			rollbackSeasonPackMoves(moved)
			return false
		}
		if err := os.Rename(plan.from, plan.to); err != nil {
			rollbackSeasonPackMoves(moved)
			zap.L().Error("合集文件归档失败", zap.Uint("id", dlID), zap.Int("episode", plan.ep), zap.Error(err))
			return false
		}
		moved = append(moved, plan)
	}

	now := time.Now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Download{}).Where("id = ?", candidate.ID).Updates(map[string]interface{}{
			"status":              model.DownloadStatusCompleted,
			"progress":            100.0,
			"completed_at":        &now,
			"file_path":           filepath.Dir(plans[0].to),
			"failure_kind":        "",
			"last_error":          "",
			"next_retry_at":       nil,
			"seeking_alternative": false,
			"quality_note":        fmt.Sprintf("合集已归档 %d 集", len(plans)),
		}).Error; err != nil {
			return err
		}
		for _, plan := range plans {
			downloadID := candidate.TorrentID
			path, size := plan.to, plan.size
			row := model.AnimeEpisode{
				AnimeID: candidateAnimeID(candidate), EpisodeNumber: plan.ep,
				Downloaded: true, DownloadID: &downloadID, FilePath: &path, FileSize: &size,
				UpdatedAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "anime_id"}, {Name: "episode_number"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"downloaded": true, "download_id": downloadID,
					"file_path": path, "file_size": size, "updated_at": now,
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		rollbackSeasonPackMoves(moved)
		zap.L().Error("合集数据库归档失败", zap.Uint("id", dlID), zap.Error(err))
		return false
	}

	s.settleSeasonPack(ctx, &candidate, episodes)
	if exec := s.executors[candidate.DownloadType]; exec != nil {
		if err := exec.Remove(executorTaskID(&candidate), false); err != nil {
			zap.L().Warn("移除已归档合集的 qBit 任务失败", zap.Uint("id", candidate.ID), zap.Error(err))
		}
	}
	removeTorrentRaceDirectory(candidate.SavePath)
	s.updateAnimeProgressFromEpisodeTable(ctx, *candidate.AnimeID)
	s.notifyCompletion(candidate.ID)
	zap.L().Info("整季合集归档完成", zap.Uint("id", candidate.ID), zap.String("name", candidate.Name), zap.Int("episodes", len(plans)))
	return true
}

func candidateAnimeID(candidate model.Download) uint {
	if candidate.AnimeID == nil {
		return 0
	}
	return *candidate.AnimeID
}

func rollbackSeasonPackMoves(moved []seasonPackMovePlan) {
	for i := len(moved) - 1; i >= 0; i-- {
		_ = os.Rename(moved[i].to, moved[i].from)
	}
}

func (s *Service) settleSeasonPack(ctx context.Context, winner *model.Download, episodes []int) {
	if winner == nil || winner.AnimeID == nil || len(episodes) == 0 {
		return
	}
	minEp, maxEp := episodes[0], episodes[len(episodes)-1]
	var siblings []model.Download
	if err := s.db.WithContext(ctx).
		Where("id <> ? AND anime_id = ? AND episode_number BETWEEN ? AND ? AND status IN ?",
			winner.ID, *winner.AnimeID, minEp, maxEp,
			[]string{model.DownloadStatusPending, model.DownloadStatusDownloading}).
		Find(&siblings).Error; err != nil {
		return
	}
	covered := make(map[int]bool, len(episodes))
	for _, ep := range episodes {
		covered[ep] = true
	}
	for i := range siblings {
		sibling := &siblings[i]
		if sibling.EpisodeNumber == nil || !covered[*sibling.EpisodeNumber] {
			continue
		}
		_ = s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", sibling.ID).Updates(map[string]interface{}{
			"status": model.DownloadStatusSuperseded, "download_speed": 0,
			"seeking_alternative": false, "quality_note": fmt.Sprintf("合集候选 #%d 已覆盖本集", winner.ID),
		}).Error
		if exec := s.executors[sibling.DownloadType]; exec != nil {
			removeFiles := sibling.DownloadType == model.DownloadTypeTorrent && sibling.SavePath != nil && IsTorrentRaceSavePath(*sibling.SavePath)
			_ = exec.Remove(executorTaskID(sibling), removeFiles)
			if removeFiles {
				removeTorrentRaceDirectory(sibling.SavePath)
			}
		}
	}
}

func (s *Service) updateAnimeProgressFromEpisodeTable(ctx context.Context, animeID uint) {
	var maxEpisode int
	if err := s.db.WithContext(ctx).Model(&model.AnimeEpisode{}).
		Where("anime_id = ? AND downloaded = ?", animeID, true).
		Select("COALESCE(MAX(episode_number), 0)").Scan(&maxEpisode).Error; err == nil {
		_ = s.db.WithContext(ctx).Model(&model.Anime{}).Where("id = ?", animeID).Update("current_episode", maxEpisode).Error
	}
}

func removeStagingFile(path string) error {
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Service) settleEpisodeRace(ctx context.Context, winner *model.Download) {
	if winner == nil || winner.AnimeID == nil || winner.EpisodeNumber == nil {
		return
	}
	var siblings []model.Download
	if err := s.db.WithContext(ctx).
		Where("id <> ? AND anime_id = ? AND episode_number = ? AND status IN ?",
			winner.ID, *winner.AnimeID, *winner.EpisodeNumber,
			[]string{model.DownloadStatusPending, model.DownloadStatusDownloading}).
		Find(&siblings).Error; err != nil {
		zap.L().Warn("查询剧集竞速候选失败", zap.Uint("winner_id", winner.ID), zap.Error(err))
		return
	}
	for i := range siblings {
		sibling := &siblings[i]
		note := fmt.Sprintf("同集候选 #%d 已完成，结束多源竞速", winner.ID)
		if err := s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", sibling.ID).
			Updates(map[string]interface{}{
				"status":              model.DownloadStatusSuperseded,
				"download_speed":      0,
				"seeking_alternative": false,
				"quality_note":        note,
			}).Error; err != nil {
			continue
		}
		if exec := s.executors[sibling.DownloadType]; exec != nil {
			removeFiles := sibling.DownloadType == model.DownloadTypeTorrent &&
				sibling.SavePath != nil && IsTorrentRaceSavePath(*sibling.SavePath)
			if err := exec.Remove(executorTaskID(sibling), removeFiles); err != nil {
				zap.L().Warn("取消竞速候选失败",
					zap.Uint("winner_id", winner.ID), zap.Uint("candidate_id", sibling.ID), zap.Error(err))
				_ = s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", sibling.ID).
					Updates(map[string]interface{}{
						"status":       sibling.Status,
						"quality_note": "竞速赢家已完成，但取消本候选失败；下轮继续收敛",
					}).Error
				continue
			}
			if removeFiles {
				removeTorrentRaceDirectory(sibling.SavePath)
			}
		}
	}
}

// qBittorrent's add API returns no hash. TorrentID therefore starts as a
// synthetic UI/runtime ID, while InfoHash is the stable identifier accepted
// by qBittorrent control APIs.
func executorTaskID(dl *model.Download) string {
	if dl != nil && dl.DownloadType == model.DownloadTypeTorrent &&
		dl.InfoHash != nil && strings.TrimSpace(*dl.InfoHash) != "" {
		return strings.TrimSpace(*dl.InfoHash)
	}
	if dl == nil {
		return ""
	}
	return dl.TorrentID
}

func removeTorrentRaceDirectory(path *string) {
	if path == nil || !IsTorrentRaceSavePath(*path) {
		return
	}
	if err := os.RemoveAll(*path); err != nil {
		zap.L().Warn("清理 BT 竞速隔离目录失败",
			zap.String("path", *path), zap.Error(err))
	}
}

// ListResult holds paginated download list results.
type ListResult struct {
	Items []model.Download `json:"items"`
	Total int64            `json:"total"`
}

// List returns a paginated list of downloads with optional filters.
func (s *Service) List(ctx context.Context, status, downloadType string, animeID uint, roadName, detailURL string, page, pageSize int) (*ListResult, error) {
	query := s.db.WithContext(ctx).Model(&model.Download{})
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("status <> ?", model.DownloadStatusSuperseded)
	}
	if downloadType != "" {
		query = query.Where("download_type = ?", downloadType)
	}
	if animeID > 0 {
		query = query.Where("anime_id = ?", animeID)
	}
	if roadName != "" {
		query = query.Where("stream_road_name = ?", roadName)
	}
	if detailURL != "" {
		query = query.Where("stream_detail_url = ?", detailURL)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var downloads []model.Download
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&downloads).Error; err != nil {
		return nil, err
	}
	return &ListResult{Items: downloads, Total: total}, nil
}

// GetByID returns a single download by ID.
func (s *Service) GetByID(ctx context.Context, id uint) (*model.Download, error) {
	var dl model.Download
	if err := s.db.WithContext(ctx).First(&dl, id).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}
	return &dl, nil
}

// Retry resets a failed download to pending.
func (s *Service) Retry(ctx context.Context, id uint) (*model.Download, error) {
	var dl model.Download
	if err := s.db.WithContext(ctx).First(&dl, id).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}
	if dl.Status != model.DownloadStatusFailed {
		return nil, fmt.Errorf("只有失败的任务可以重试")
	}

	// 重置状态与进度
	s.db.WithContext(ctx).Model(&dl).Updates(map[string]interface{}{
		"status":           model.DownloadStatusPending,
		"progress":         0,
		"downloaded_bytes": nil,
	})
	dl.Status = model.DownloadStatusPending

	// 真正重新执行（stream 类型）
	if dl.DownloadType == model.DownloadTypeStream {
		if dl.StreamRuleID == nil {
			s.updateStatus(dl.ID, model.DownloadStatusFailed, nil)
			return nil, fmt.Errorf("stream 任务缺少规则")
		}
		var rule model.StreamRule
		if err := s.db.WithContext(ctx).First(&rule, *dl.StreamRuleID).Error; err != nil {
			s.updateStatus(dl.ID, model.DownloadStatusFailed, nil)
			return nil, fmt.Errorf("规则不存在")
		}
		task := &Task{
			Name:          dl.Name,
			URL:           dl.URL,
			DownloadType:  dl.DownloadType,
			Source:        dl.Source,
			AnimeID:       dl.AnimeID,
			EpisodeNumber: dl.EpisodeNumber,
			StreamRuleID:  dl.StreamRuleID,
			StreamRule:    &rule,
			AnimeName:     dl.Name,
		}
		if dl.StreamRoadName != nil {
			task.StreamRoadName = *dl.StreamRoadName
		}
		if dl.StreamDetailURL != nil {
			task.StreamDetailURL = *dl.StreamDetailURL
		}
		if dl.SavePath != nil {
			task.SavePath = *dl.SavePath
		}
		go s.execute(dl.ID, dl.TorrentID, task)
	}
	return &dl, nil
}

// PauseAll pauses all active torrent downloads.
func (s *Service) PauseAll(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Model(&model.Download{}).
		Where("status IN ? AND download_type = ?", []string{model.DownloadStatusDownloading, model.DownloadStatusPending}, model.DownloadTypeTorrent).
		Update("status", model.DownloadStatusPaused)
	return result.RowsAffected, result.Error
}

// ResumeAll resumes all paused torrent downloads.
func (s *Service) ResumeAll(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Model(&model.Download{}).
		Where("status = ? AND download_type = ?", model.DownloadStatusPaused, model.DownloadTypeTorrent).
		Update("status", model.DownloadStatusDownloading)
	return result.RowsAffected, result.Error
}

// RecoverPending 服务启动时恢复未完成的下载任务。
// 重启前状态为 pending/downloading 的任务，重启后执行 goroutine 已丢失，
// 这里重新调度执行（stream 类型）。torrent 由下载器自身管理，不处理。
func (s *Service) RecoverPending(ctx context.Context) {
	s.normalizeHistoricalFailureKinds(ctx)
	s.resolveHistoricalFailures(ctx)
	s.reconcileCompletedEpisodeStates(ctx)
	// 启动时做一次性数据迁移：历史 stream 下载的 stream_road_name 为 NULL，
	// 用 anime 当前的 stream_road_name 回填（通常是"播放列表1"）
	s.migrateStreamRoadName(ctx)

	var downloads []model.Download
	if err := s.db.WithContext(ctx).
		Where("status IN ? AND download_type = ?",
			[]string{model.DownloadStatusPending, model.DownloadStatusDownloading},
			model.DownloadTypeStream).
		Find(&downloads).Error; err != nil {
		zap.L().Error("查询未完成下载任务失败", zap.Error(err))
		return
	}

	if len(downloads) == 0 {
		return
	}

	zap.L().Info("恢复未完成的下载任务", zap.Int("count", len(downloads)))

	for i := range downloads {
		dl := &downloads[i]
		// 重置进度
		s.db.Model(dl).Updates(map[string]interface{}{
			"status":   model.DownloadStatusPending,
			"progress": 0,
		})

		// 构造 Task 并执行
		var rule model.StreamRule
		if dl.StreamRuleID == nil {
			zap.L().Warn("stream 下载无规则 ID，标记失败", zap.Uint("id", dl.ID))
			s.updateStatus(dl.ID, model.DownloadStatusFailed, nil)
			continue
		}
		if err := s.db.WithContext(ctx).First(&rule, *dl.StreamRuleID).Error; err != nil {
			zap.L().Warn("获取规则失败，标记下载失败", zap.Uint("id", dl.ID), zap.Error(err))
			s.updateStatus(dl.ID, model.DownloadStatusFailed, nil)
			continue
		}

		task := &Task{
			Name:          dl.Name,
			URL:           dl.URL,
			DownloadType:  dl.DownloadType,
			Source:        dl.Source,
			AnimeID:       dl.AnimeID,
			EpisodeNumber: dl.EpisodeNumber,
			StreamRuleID:  dl.StreamRuleID,
			StreamRule:    &rule,
			AnimeName:     dl.Name,
		}
		if dl.SavePath != nil {
			task.SavePath = *dl.SavePath
		}

		go s.execute(dl.ID, dl.TorrentID, task)
	}
}

// normalizeHistoricalFailureKinds 修正旧版本遗留的错误分类。早期版本把季度/
// 集数不匹配也记成 transient，RetryFailedJob 因而会周期性复活已被主动删除的
// qBit 任务。启动时将这类记录改为候选级 rejected，并清除冷却时间。
func (s *Service) normalizeHistoricalFailureKinds(ctx context.Context) {
	var rows []model.Download
	if err := s.db.WithContext(ctx).
		Where("status = ? AND last_error <> ''", model.DownloadStatusFailed).
		Where("failure_kind IN ? OR failure_kind = ''", []string{model.FailureKindTransient}).
		Find(&rows).Error; err != nil {
		zap.L().Warn("查询历史失败分类失败", zap.Error(err))
		return
	}

	var updated int64
	for i := range rows {
		kind, _ := classifyError(fmt.Errorf("%s", rows[i].LastError), rows[i].RetryCount)
		if kind != model.FailureKindRejected {
			continue
		}
		result := s.db.WithContext(ctx).Model(&model.Download{}).
			Where("id = ?", rows[i].ID).
			Updates(func() map[string]interface{} {
				updates := rejectedCandidateCleanup()
				updates["failure_kind"] = model.FailureKindRejected
				return updates
			}())
		if result.Error != nil {
			zap.L().Warn("修正历史失败分类失败", zap.Uint("id", rows[i].ID), zap.Error(result.Error))
			continue
		}
		updated += result.RowsAffected
	}
	if updated > 0 {
		zap.L().Info("已修正历史候选错误分类", zap.Int64("rows", updated))
	}
}

// updateStatus updates a download's status in the DB.
// migrateStreamRoadName 历史 stream 下载无 road_name，用 anime.stream_road_name 回填。
func (s *Service) migrateStreamRoadName(ctx context.Context) {
	result := s.db.WithContext(ctx).Exec(`
		UPDATE download
		SET stream_road_name = a.stream_road_name,
		    stream_detail_url = COALESCE(download.stream_detail_url, a.stream_detail_url)
		FROM anime a
		WHERE download.anime_id = a.id
		  AND download.download_type = 'stream'
		  AND (download.stream_road_name IS NULL OR download.stream_detail_url IS NULL)
		  AND a.stream_road_name IS NOT NULL
	`)
	if result.Error != nil {
		zap.L().Warn("回填历史下载 road_name 失败", zap.Error(result.Error))
		return
	}
	if result.RowsAffected > 0 {
		zap.L().Info("回填历史下载 road_name/detail_url 完成", zap.Int64("rows", result.RowsAffected))
	}
}

func (s *Service) updateStatus(dlID uint, status string, extra map[string]interface{}) {
	// 在 UPDATE 之前先读旧状态：只有"原本不是 completed"翻成"现在是 completed"
	// 才发通知（防止重复推送）。读失败不影响主流程。
	var prev model.Download
	hadRow := s.db.First(&prev, dlID).Error == nil

	updates := map[string]interface{}{"status": status}
	if status == model.DownloadStatusCompleted {
		now := time.Now()
		updates["completed_at"] = &now
	}
	for k, v := range extra {
		updates[k] = v
	}
	if err := s.db.Model(&model.Download{}).Where("id = ?", dlID).Updates(updates).Error; err != nil {
		zap.L().Error("更新下载状态失败",
			zap.Uint("id", dlID),
			zap.String("status", status),
			zap.Error(err))
		return
	}

	// stream 下载完成时，更新 anime 的 current_episode 为最大集数
	if status == model.DownloadStatusCompleted {
		s.updateAnimeProgress(dlID)
		s.resolvePriorFailures(dlID)

		// 首次翻成 completed 才发通知（去重核心：prev.Status != completed）
		if hadRow && prev.Status != model.DownloadStatusCompleted {
			s.notifyCompletion(dlID)
		}
	}
}

// resolvePriorFailures 将已被同集后续成功任务解决的失败尝试收口为 superseded。
// 记录仍保留用于审计，但默认下载列表和自动重试不再把它视为当前故障。
func (s *Service) resolvePriorFailures(completedID uint) {
	var completed model.Download
	if err := s.db.First(&completed, completedID).Error; err != nil || completed.AnimeID == nil || completed.EpisodeNumber == nil {
		return
	}
	s.db.Model(&model.Download{}).
		Where("id <> ? AND anime_id = ? AND episode_number = ? AND status = ?",
			completed.ID, *completed.AnimeID, *completed.EpisodeNumber, model.DownloadStatusFailed).
		Update("status", model.DownloadStatusSuperseded)
}

func (s *Service) resolveHistoricalFailures(ctx context.Context) {
	result := s.db.WithContext(ctx).Exec(`
		UPDATE download AS failed
		SET status = ?
		WHERE failed.status = ?
		  AND failed.anime_id IS NOT NULL
		  AND failed.episode_number IS NOT NULL
		  AND EXISTS (
			SELECT 1 FROM download AS done
			WHERE done.anime_id = failed.anime_id
			  AND done.episode_number = failed.episode_number
			  AND done.status = ?
		  )`, model.DownloadStatusSuperseded, model.DownloadStatusFailed, model.DownloadStatusCompleted)
	if result.Error != nil {
		zap.L().Warn("收口历史失败任务失败", zap.Error(result.Error))
	} else if result.RowsAffected > 0 {
		zap.L().Info("历史失败任务已标记为已解决", zap.Int64("rows", result.RowsAffected))
	}
}

// reconcileCompletedEpisodeStates repairs two kinds of historical drift:
//  1. old qBit sync versions could turn superseded seeding tasks back into
//     completed, leaving more than one winner for the same episode;
//  2. a completed download did not always update animeepisode, so the UI
//     could still show the episode as missing even though the media existed.
//
// Prefer a completed row carrying a concrete media path as the winner. When
// no row has one, keep the earliest completion (then the lowest id) so the
// result is deterministic. This only changes database state; media files are
// never removed here.
func (s *Service) reconcileCompletedEpisodeStates(ctx context.Context) {
	var completed []model.Download
	if err := s.db.WithContext(ctx).
		Where("status = ? AND media_missing = ? AND anime_id IS NOT NULL AND episode_number IS NOT NULL",
			model.DownloadStatusCompleted, false).
		Order("anime_id ASC, episode_number ASC, completed_at ASC, id ASC").
		Find(&completed).Error; err != nil {
		zap.L().Warn("查询历史完成剧集失败", zap.Error(err))
		return
	}

	type episodeKey struct {
		animeID uint
		episode int
	}
	groups := make(map[episodeKey][]model.Download)
	for i := range completed {
		dl := completed[i]
		groups[episodeKey{animeID: *dl.AnimeID, episode: *dl.EpisodeNumber}] = append(
			groups[episodeKey{animeID: *dl.AnimeID, episode: *dl.EpisodeNumber}], dl)
	}

	repairedEpisodes := 0
	superseded := 0
	for _, group := range groups {
		winnerIndex := 0
		for i := range group {
			if group[i].FilePath != nil && strings.TrimSpace(*group[i].FilePath) != "" {
				winnerIndex = i
				break
			}
		}
		winner := &group[winnerIndex]
		if err := s.syncCompletedEpisodeState(ctx, winner); err != nil {
			zap.L().Warn("修复已完成剧集状态失败",
				zap.Uint("download_id", winner.ID), zap.Error(err))
			continue
		}
		repairedEpisodes++

		for i := range group {
			if i == winnerIndex {
				continue
			}
			result := s.db.WithContext(ctx).Model(&model.Download{}).
				Where("id = ? AND status = ?", group[i].ID, model.DownloadStatusCompleted).
				Updates(map[string]interface{}{
					"status":       model.DownloadStatusSuperseded,
					"quality_note": "同集已有完成记录，历史重复状态已收口",
				})
			if result.Error != nil {
				zap.L().Warn("收口历史重复完成记录失败",
					zap.Uint("download_id", group[i].ID), zap.Error(result.Error))
				continue
			}
			superseded += int(result.RowsAffected)
		}
	}

	if repairedEpisodes > 0 || superseded > 0 {
		zap.L().Info("历史下载状态校准完成",
			zap.Int("episodes", repairedEpisodes), zap.Int("superseded", superseded))
	}
}

func (s *Service) syncCompletedEpisodeState(ctx context.Context, dl *model.Download) error {
	if dl == nil || dl.AnimeID == nil || dl.EpisodeNumber == nil || *dl.EpisodeNumber <= 0 {
		return nil
	}
	now := time.Now()
	downloadID := dl.TorrentID
	row := model.AnimeEpisode{
		AnimeID:       *dl.AnimeID,
		EpisodeNumber: *dl.EpisodeNumber,
		Downloaded:    true,
		DownloadID:    &downloadID,
		UpdatedAt:     now,
	}
	updates := map[string]interface{}{
		"downloaded":  true,
		"download_id": downloadID,
		"updated_at":  now,
	}
	if dl.FilePath != nil && strings.TrimSpace(*dl.FilePath) != "" {
		path := strings.TrimSpace(*dl.FilePath)
		row.FilePath = &path
		updates["file_path"] = path
	}
	if dl.TotalBytes != nil && *dl.TotalBytes > 0 {
		size := *dl.TotalBytes
		row.FileSize = &size
		updates["file_size"] = size
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "anime_id"}, {Name: "episode_number"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&row).Error
}

// updateAnimeProgress 在下载完成时更新 anime.current_episode（所有源类型通用）
func (s *Service) updateAnimeProgress(dlID uint) {
	var dl model.Download
	if err := s.db.First(&dl, dlID).Error; err != nil {
		zap.L().Error("查询下载记录失败", zap.Uint("id", dlID), zap.Error(err))
		return
	}
	if dl.AnimeID == nil {
		return
	}
	if err := s.syncCompletedEpisodeState(context.Background(), &dl); err != nil {
		zap.L().Warn("同步已完成剧集状态失败", zap.Uint("download_id", dlID), zap.Error(err))
	}

	// 查询该 anime 所有已完成的集数（跨所有 source/download_type），取最大值
	var maxEpisode int
	err := s.db.Model(&model.Download{}).
		Select("COALESCE(MAX(episode_number), 0)").
		Where("anime_id = ? AND status = ?", *dl.AnimeID, model.DownloadStatusCompleted).
		Scan(&maxEpisode).Error
	if err != nil {
		zap.L().Error("查询 anime 进度失败", zap.Uint("anime_id", *dl.AnimeID), zap.Error(err))
		return
	}

	// 同时动态补全 episode_count：如果目前已知集数 < 刚下完的集数，扩展为刚下完的集数
	updates := map[string]interface{}{"current_episode": maxEpisode}
	var anime model.Anime
	if err := s.db.First(&anime, *dl.AnimeID).Error; err == nil {
		if anime.EpisodeCount == nil || *anime.EpisodeCount < maxEpisode {
			updates["episode_count"] = maxEpisode
		}
	}
	s.db.Model(&model.Anime{}).Where("id = ?", *dl.AnimeID).Updates(updates)

	zap.L().Info("更新 anime 进度",
		zap.Uint("anime_id", *dl.AnimeID),
		zap.Int("current_episode", maxEpisode),
		zap.Any("updates", updates))
}

// notifyCompletion 在某条 download 翻成 completed 时推一条通知。
//
// 设计：
//   - notifSvc 没注入 → 直接 no-op
//   - 异步 fire-and-forget：开 goroutine + 30s 独立超时 ctx，避免阻塞 updateStatus
//   - 信息组装"宽容"：anime 关联可能为空（手动下载没绑番），那时就用 dl.Name 当标题
//   - 调用方需保证 dlID 已在 DB 存在（updateStatus 里 UPDATE 成功后才走到这里）
func (s *Service) notifyCompletion(dlID uint) {
	if s.notifSvc == nil {
		return
	}

	// 同步读 dl 拿最新字段（completed_at 等已写入），构造 NotificationInfo
	var dl model.Download
	if err := s.db.First(&dl, dlID).Error; err != nil {
		zap.L().Warn("查询下载用于发通知失败", zap.Uint("id", dlID), zap.Error(err))
		return
	}

	info := &notification.NotificationInfo{}
	if dl.AnimeID != nil && *dl.AnimeID > 0 {
		var a model.Anime
		if err := s.db.First(&a, *dl.AnimeID).Error; err == nil {
			if a.OfficialTitle != nil && *a.OfficialTitle != "" {
				info.OfficialTitle = *a.OfficialTitle
			} else {
				info.OfficialTitle = a.Title
			}
			if a.Season != nil {
				info.Season = *a.Season
			} else {
				info.Season = 1
			}
			if a.CoverURL != nil && *a.CoverURL != "" {
				info.CoverURL = *a.CoverURL
			}
		}
	}
	if info.OfficialTitle == "" {
		info.OfficialTitle = dl.Name
	}
	if info.Season == 0 {
		info.Season = 1
	}
	if dl.EpisodeNumber != nil {
		info.Episode = *dl.EpisodeNumber
	}

	// 幂等闸门：按 (anime_id, episode_number) 去重，无论触发路径是
	// BT 完成、Stream 备援完成、qbit_sync 误标 completed 还是 dl 行被
	// 删后重建 —— 同一集只发一次。
	animeID := uint(0)
	if dl.AnimeID != nil {
		animeID = *dl.AnimeID
	}
	if !claimEpisodeNotification(s.db, animeID, info.Episode, info.Season) {
		return
	}

	go func(info *notification.NotificationInfo) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.notifSvc.Broadcast(ctx, info)
	}(info)
}

// generateTorrentID produces a consistent, type-prefixed ID.
func generateTorrentID(downloadType string) string {
	short := uuid.New().String()[:8]
	switch downloadType {
	case model.DownloadTypeStream:
		return "stream_" + short
	case model.DownloadTypeTorrent:
		return "torrent_" + short
	default:
		return short
	}
}

// resolveSavePath determines the save path for a task.
func resolveSavePath(cfg *config.Config, task *Task) *string {
	if task.SavePath != "" {
		return &task.SavePath
	}
	switch task.DownloadType {
	case model.DownloadTypeStream:
		if cfg.StreamDownloadDir != "" {
			return &cfg.StreamDownloadDir
		}
		fallthrough
	case model.DownloadTypeTorrent:
		if cfg.MediaRoot != "" {
			dir := filepath.Join(cfg.MediaRoot, task.DownloadType)
			return &dir
		}
	}
	return nil
}

// resolveTaskSavePath 把所有带番剧和集数信息的 BT 任务统一放进隔离目录。
// Orchestrator、RSS 和手动选种最终都会经过 Service.Create，因此不会再因
// 入口不同而绕过完成归档。调用方显式提供的路径仍优先保留。
func (s *Service) resolveTaskSavePath(ctx context.Context, task *Task) *string {
	if task.SavePath != "" {
		return &task.SavePath
	}
	if task.DownloadType == model.DownloadTypeTorrent && task.AnimeID != nil && task.Scope == model.DownloadScopeSeason &&
		task.EpisodeStart != nil && task.EpisodeEnd != nil {
		path := BuildTorrentPackSavePath(
			s.currentMediaRoot(ctx),
			*task.AnimeID,
			*task.EpisodeStart,
			*task.EpisodeEnd,
			ExtractInfoHash(task.URL),
			task.URL,
		)
		return &path
	}
	if task.DownloadType == model.DownloadTypeTorrent && task.AnimeID != nil && task.EpisodeNumber != nil {
		path := BuildTorrentRaceSavePath(
			s.currentMediaRoot(ctx),
			*task.AnimeID,
			*task.EpisodeNumber,
			ExtractInfoHash(task.URL),
			task.URL,
		)
		return &path
	}
	return resolveSavePath(s.cfg, task)
}

func (s *Service) currentMediaRoot(ctx context.Context) string {
	for _, key := range []string{"download_dir", "media_root"} {
		var setting model.Setting
		if err := s.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err == nil {
			if value := strings.TrimSpace(setting.Value); value != "" {
				return value
			}
		}
	}
	if value := strings.TrimSpace(s.cfg.MediaRoot); value != "" {
		return value
	}
	return "/downloads"
}

// MediaRoot 返回当前真正生效的媒体根目录，供系统状态和其他只读探针使用。
func (s *Service) MediaRoot(ctx context.Context) string {
	return s.currentMediaRoot(ctx)
}

// ensureMediaCapacity 在管理员配置阈值后统一保护所有下载入口。
// 阈值为 0 或未配置表示不启用对应限制。
func (s *Service) ensureMediaCapacity(ctx context.Context) error {
	readFloat := func(key string) float64 {
		var setting model.Setting
		if err := s.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
			return 0
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(setting.Value), 64)
		if err != nil || value <= 0 {
			return 0
		}
		return value
	}
	minFreeGB := readFloat("media.min_free_gb")
	maxUsedPercent := readFloat("media.max_used_percent")
	if minFreeGB == 0 && maxUsedPercent == 0 {
		return nil
	}

	root := s.currentMediaRoot(ctx)
	usage, err := disk.Usage(root)
	if err != nil {
		return fmt.Errorf("无法读取 %s 的容量: %w", root, err)
	}
	if maxUsedPercent > 0 && usage.UsedPercent >= maxUsedPercent {
		return fmt.Errorf("%s 已使用 %.1f%%，达到 %.1f%% 上限", root, usage.UsedPercent, maxUsedPercent)
	}
	freeGB := float64(usage.Free) / float64(1024*1024*1024)
	if minFreeGB > 0 && freeGB < minFreeGB {
		return fmt.Errorf("%s 仅剩 %.1f GB，低于 %.1f GB 保留空间", root, freeGB, minFreeGB)
	}
	return nil
}
