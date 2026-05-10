package download

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
)

// StreamExecutor executes stream downloads using rod + ffmpeg.
type StreamExecutor struct {
	streamMgr *stream.StreamManager
	db        *gorm.DB
}

// NewStreamExecutor creates a new stream executor.
func NewStreamExecutor(mgr *stream.StreamManager, db *gorm.DB) *StreamExecutor {
	return &StreamExecutor{streamMgr: mgr, db: db}
}

// Execute runs a stream download via the StreamManager pipeline.
func (e *StreamExecutor) Execute(ctx context.Context, task *Task, progressCB ProgressCallback) (*Result, error) {
	if task.StreamRule == nil {
		return nil, fmt.Errorf("stream task missing StreamRule")
	}

	ep := stream.EpisodeInfo{
		Name: task.Name,
		URL:  task.URL,
	}

	// 获取 anime 以构建 Plex/Emby 规范路径
	var anime *model.Anime
	if task.AnimeID != nil && e.db != nil {
		var a model.Anime
		if err := e.db.WithContext(ctx).First(&a, *task.AnimeID).Error; err == nil {
			anime = &a
		}
	}

	episodeNumber := 0
	if task.EpisodeNumber != nil {
		episodeNumber = *task.EpisodeNumber
	}

	// Adapt our 3-arg ProgressCallback to StreamManager's 2-arg callback
	streamCB := func(progress float64, downloadedBytes int64) {
		if progressCB != nil {
			progressCB(progress, downloadedBytes, 0)
		}
	}

	filePath, err := e.streamMgr.DownloadEpisode(ctx, &ep, task.StreamRule, task.SavePath, task.AnimeName, anime, episodeNumber, streamCB)
	if err != nil {
		return nil, err
	}

	return &Result{FilePath: filePath}, nil
}

// Cancel cancels an in-progress stream download.
func (e *StreamExecutor) Cancel(taskID string) error {
	e.streamMgr.CancelDownload(taskID)
	return nil
}

// Pause is not supported for stream downloads.
func (e *StreamExecutor) Pause(taskID string) error {
	return fmt.Errorf("流媒体下载不支持暂停")
}

// Resume is not supported for stream downloads.
func (e *StreamExecutor) Resume(taskID string) error {
	return fmt.Errorf("流媒体下载不支持恢复")
}

// Remove cancels the download.
func (e *StreamExecutor) Remove(taskID string, removeFiles bool) error {
	e.streamMgr.CancelDownload(taskID)
	return nil
}
