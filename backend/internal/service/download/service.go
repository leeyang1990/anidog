package download

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/notification"
	"github.com/anidog/anidog-go/internal/ws"
)

// Service is the unified download service. All download creation and
// execution goes through this, regardless of trigger source.
type Service struct {
	db        *gorm.DB
	cfg       *config.Config
	hub       *ws.Hub
	executors map[string]Executor
	notifSvc  *notification.Service // 可空：未注入时不发通知
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

// SetNotificationService 注入通知服务。
// 这是唯一的通知收口：所有下载完成事件（不管是 BT/Stream/Manual 哪条路径触发）
// 都从 updateStatus → notifyCompletion 走，避免在多处事件源各自接钩子导致漏发或重发。
func (s *Service) SetNotificationService(n *notification.Service) {
	s.notifSvc = n
}

// Create creates a Download record and starts async execution.
func (s *Service) Create(ctx context.Context, task *Task) (*model.Download, error) {
	if err := task.Validate(); err != nil {
		return nil, fmt.Errorf("invalid task: %w", err)
	}

	torrentID := generateTorrentID(task.DownloadType)
	savePath := resolveSavePath(s.cfg, task)

	dl := model.Download{
		TorrentID:      torrentID,
		Name:           task.Name,
		URL:            task.URL,
		SavePath:       savePath,
		Status:         model.DownloadStatusPending,
		DownloadType:   task.DownloadType,
		StreamRuleID:   task.StreamRuleID,
		AnimeID:        task.AnimeID,
		EpisodeNumber:  task.EpisodeNumber,
		Source:         task.Source,
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

	if err := exec.Cancel(dl.TorrentID); err != nil {
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

	if err := exec.Pause(dl.TorrentID); err != nil {
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

	if err := exec.Resume(dl.TorrentID); err != nil {
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
		if err := exec.Remove(dl.TorrentID, removeFiles); err != nil {
			zap.L().Warn("移除下载文件失败", zap.Error(err))
		}
	}

	return s.db.Delete(&dl).Error
}

// execute runs the download in a goroutine, updating DB state as it goes.
func (s *Service) execute(dlID uint, torrentID string, task *Task) {
	exec, ok := s.executors[task.DownloadType]
	if !ok {
		s.updateStatus(dlID, model.DownloadStatusFailed, nil)
		zap.L().Error("无对应下载执行器", zap.String("type", task.DownloadType))
		return
	}

	s.updateStatus(dlID, model.DownloadStatusDownloading, nil)

	progressCB := func(progress float64, downloadedBytes, totalBytes int64) {
		updates := map[string]interface{}{
			"progress": progress,
		}
		if downloadedBytes > 0 {
			updates["downloaded_bytes"] = downloadedBytes
		}
		if totalBytes > 0 {
			updates["total_bytes"] = totalBytes
		}
		s.db.Model(&model.Download{}).Where("id = ?", dlID).Updates(updates)

		if s.hub != nil {
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
		s.updateStatus(dlID, model.DownloadStatusFailed, nil)
		zap.L().Error("下载失败", zap.String("name", task.Name), zap.Error(err))
		return
	}

	extra := map[string]interface{}{
		"progress": 100.0,
	}
	if result != nil {
		if result.FilePath != "" {
			extra["file_path"] = result.FilePath
		}
		// 不覆盖 torrent_id：该字段是 uniqueIndex，Create 时生成的值是唯一 ID；
		// provider 返回的 hash（或占位 "new_torrent"）如果覆盖会导致 UNIQUE 冲突，
		// 整个 UPDATE 失败，状态卡在 downloading。真正的 BT info hash 应走单独字段存。
	}
	s.updateStatus(dlID, model.DownloadStatusCompleted, extra)

	if s.hub != nil {
		s.hub.BroadcastDownloadComplete(torrentID, task.Name)
	}
	zap.L().Info("下载完成", zap.String("name", task.Name))
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
		"status":            model.DownloadStatusPending,
		"progress":          0,
		"downloaded_bytes":  nil,
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

		// 首次翻成 completed 才发通知（去重核心：prev.Status != completed）
		if hadRow && prev.Status != model.DownloadStatusCompleted {
			s.notifyCompletion(dlID)
		}
	}
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
