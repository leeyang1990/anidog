package download

import (
	"context"
	"fmt"

	"github.com/anidog/anidog-go/internal/service"
)

// TorrentExecutor executes torrent downloads via qBittorrent.
type TorrentExecutor struct {
	client service.Downloader
}

// NewTorrentExecutor creates a new torrent executor.
func NewTorrentExecutor(client service.Downloader) *TorrentExecutor {
	return &TorrentExecutor{client: client}
}

// Execute adds a torrent to qBittorrent and returns the hash.
func (e *TorrentExecutor) Execute(ctx context.Context, task *Task, progressCB ProgressCallback) (*Result, error) {
	savePath := ""
	if task.SavePath != "" {
		savePath = task.SavePath
	}

	hash, err := e.client.AddTorrent(ctx, task.URL, savePath)
	if err != nil {
		return nil, fmt.Errorf("添加种子失败: %w", err)
	}

	return &Result{TorrentID: hash}, nil
}

// Cancel removes the torrent from qBittorrent.
func (e *TorrentExecutor) Cancel(taskID string) error {
	return e.client.RemoveTorrent(context.Background(), taskID, true)
}

// Pause pauses the torrent in qBittorrent.
func (e *TorrentExecutor) Pause(taskID string) error {
	return e.client.PauseTorrent(context.Background(), taskID)
}

// Resume resumes the torrent in qBittorrent.
func (e *TorrentExecutor) Resume(taskID string) error {
	return e.client.ResumeTorrent(context.Background(), taskID)
}

// Remove removes the torrent from qBittorrent.
func (e *TorrentExecutor) Remove(taskID string, removeFiles bool) error {
	return e.client.RemoveTorrent(context.Background(), taskID, removeFiles)
}
