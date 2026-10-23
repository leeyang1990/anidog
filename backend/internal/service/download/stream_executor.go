package download

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
)

// StreamExecutor executes stream downloads using rod + ffmpeg.
type StreamExecutor struct {
	streamMgr *stream.StreamManager
	db        *gorm.DB
	catalog   streamSourceCatalog
	active    sync.Map
}

type streamSourceCatalog interface {
	SearchAnime(context.Context, *model.StreamRule, string) ([]stream.SearchResult, error)
	GetEpisodes(context.Context, *model.StreamRule, string) ([]stream.EpisodeInfo, error)
}

// NewStreamExecutor creates a new stream executor.
func NewStreamExecutor(mgr *stream.StreamManager, db *gorm.DB) *StreamExecutor {
	return &StreamExecutor{streamMgr: mgr, db: db, catalog: mgr}
}

// Execute runs a stream download via the StreamManager pipeline.
func (e *StreamExecutor) Execute(ctx context.Context, task *Task, progressCB ProgressCallback) (*Result, error) {
	if task.StreamRule == nil {
		return nil, fmt.Errorf("stream task missing StreamRule")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	e.active.Store(task.RuntimeID, cancel)
	defer e.active.Delete(task.RuntimeID)

	ep := stream.EpisodeInfo{
		Name: task.Name,
		URL:  task.URL,
	}

	// 获取 anime 以构建 Plex/Emby 规范路径
	var anime *model.Anime
	if task.AnimeID != nil && e.db != nil {
		var a model.Anime
		if err := e.db.WithContext(ctx).First(&a, *task.AnimeID).Error; err != nil {
			return nil, fmt.Errorf("读取流媒体目标番剧失败: %w", err)
		}
		anime = &a
	}
	if err := e.validateSource(ctx, task, anime); err != nil {
		return nil, err
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

	filePath, finalPath, err := e.streamMgr.DownloadEpisode(
		ctx, task.RuntimeID, &ep, task.StreamRule, task.SavePath,
		task.AnimeName, anime, episodeNumber, streamCB, task.Preflight)
	if err != nil {
		return nil, err
	}

	return &Result{FilePath: filePath, FinalPath: finalPath}, nil
}

// Revalidate at execution, including explicit retries and restart recovery.
// Old SourceTitle values may be copied from the subscription rather than the
// provider; they cannot establish the identity of a persisted playback URL.
func (e *StreamExecutor) validateSource(ctx context.Context, task *Task, anime *model.Anime) error {
	if task.Source == SourceManual || task.AnimeID == nil {
		return nil
	}
	if anime == nil || task.EpisodeNumber == nil || *task.EpisodeNumber <= 0 || task.StreamDetailURL == "" {
		return fmt.Errorf("流媒体候选无效：缺少源身份信息，请重新选源")
	}
	if e.catalog == nil {
		return fmt.Errorf("流媒体源身份复核失败：目录服务不可用")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	results, err := e.catalog.SearchAnime(ctx, task.StreamRule, anime.Title)
	if err != nil {
		return fmt.Errorf("流媒体源身份复核失败：%w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("流媒体源身份复核失败：目录暂未返回候选，等待重新搜索")
	}
	matched := false
	for _, result := range results {
		if result.URL == task.StreamDetailURL && stream.IsConfidentMatch(anime.Title, result.Name) {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("流媒体候选无效：目录标题无法确认目标季度/篇章，请重新选源")
	}
	episodes, err := e.catalog.GetEpisodes(ctx, task.StreamRule, task.StreamDetailURL)
	if err != nil {
		return fmt.Errorf("流媒体源身份复核失败：%w", err)
	}
	if anime.EpisodeCount != nil {
		episodes = stream.EpisodeRoadsWithinCount(episodes, *anime.EpisodeCount)
	}
	roads := make(map[string][]stream.EpisodeInfo)
	for _, episode := range episodes {
		if task.StreamRoadName == "" || episode.RoadName == task.StreamRoadName {
			roads[episode.RoadName] = append(roads[episode.RoadName], episode)
		}
	}
	for _, road := range roads {
		if episode, ok := stream.FindEpisode(road, *task.EpisodeNumber); ok && episode.URL == task.URL {
			return nil
		}
	}
	return fmt.Errorf("流媒体候选无效：播放列表编号超出目标总集数，或播放页与目标集数不匹配，请重新选源")
}

// Cancel cancels an in-progress stream download.
func (e *StreamExecutor) Cancel(taskID string) error {
	if cancel, ok := e.active.Load(taskID); ok {
		cancel.(context.CancelFunc)()
	}
	if e.streamMgr != nil {
		e.streamMgr.CancelDownload(taskID)
	}
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
	return e.Cancel(taskID)
}
