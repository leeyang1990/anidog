package download

import "context"

// ProgressCallback reports download progress.
// progress is 0-100, downloadedBytes and totalBytes may be 0 if unknown.
type ProgressCallback func(progress float64, downloadedBytes, totalBytes int64)

// Result is returned by Executor.Execute on success.
type Result struct {
	FilePath string // Set by stream executor; empty for torrent
	TorrentID string // Set by torrent executor (qBittorrent hash); empty for stream
}

// Executor executes a download. Implementations: qBittorrent (torrent), ffmpeg (stream).
type Executor interface {
	// Execute runs the download. Blocks until done or error.
	Execute(ctx context.Context, task *Task, progressCB ProgressCallback) (*Result, error)

	// Cancel cancels an in-progress download identified by taskID.
	Cancel(taskID string) error

	// Pause pauses the download. Returns error if unsupported.
	Pause(taskID string) error

	// Resume resumes a paused download. Returns error if unsupported.
	Resume(taskID string) error

	// Remove removes the download and optionally its files.
	Remove(taskID string, removeFiles bool) error
}
