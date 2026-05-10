package download

import (
	"fmt"

	"github.com/anidog/anidog-go/internal/model"
)

// Source constants identify the trigger that created a download.
const (
	SourceManual  = "manual"
	SourceStream  = "stream"
	SourceBangumi = "bangumi"
	SourceRSS     = "rss"
	SourceBT      = "bt"
)

// Task represents a download to be executed. Created by handlers/services,
// converted to a model.Download record by Service.
type Task struct {
	// Name is the human-readable name (e.g. "Frieren - 第1集")
	Name string

	// URL is the resource URL. For torrents: magnet/torrent URL.
	// For streams: the episode page URL (executor will intercept the video URL).
	URL string

	// DownloadType is "torrent" or "stream".
	DownloadType string

	// SavePath is the output directory. Empty = use config default.
	SavePath string

	// Source identifies the trigger: use SourceManual / SourceStream / SourceBangumi / SourceRSS.
	Source string

	// Optional relations
	AnimeID       *uint
	EpisodeNumber *int
	StreamRuleID  *uint
	StreamDetailURL string  // 详情页 URL（同一 anime 不同候选区分用）
	StreamRoadName string  // 清单名（Plex/Emby 不需要但我们用来区分完成状态）

	// Stream-specific: the rule needed by the stream executor.
	// Set only when DownloadType == "stream".
	StreamRule *model.StreamRule

	// Stream-specific: anime name for file naming.
	AnimeName string
}

// Validate checks that the Task has all required fields.
func (t *Task) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if t.URL == "" {
		return fmt.Errorf("task URL is required")
	}
	if t.DownloadType != model.DownloadTypeTorrent && t.DownloadType != model.DownloadTypeStream {
		return fmt.Errorf("invalid download type: %s", t.DownloadType)
	}
	validSources := map[string]bool{
		SourceManual: true, SourceStream: true, SourceBangumi: true, SourceRSS: true, SourceBT: true,
	}
	if !validSources[t.Source] {
		return fmt.Errorf("invalid source: %s", t.Source)
	}
	if t.DownloadType == model.DownloadTypeStream && t.StreamRule == nil {
		return fmt.Errorf("stream task requires StreamRule")
	}
	return nil
}
