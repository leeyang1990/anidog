package mock

import (
	"context"
	

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/downloader"
)

type MockDownloader struct{}

func NewProvider(cfg *config.Config) (downloader.Downloader, error) {
	return &MockDownloader{}, nil
}

func (m *MockDownloader) AddTorrent(ctx context.Context, torrentURL, savePath string) (string, error) {
	return "mock_torrent_id", nil
}

func (m *MockDownloader) PauseTorrent(ctx context.Context, torrentID string) error {
	return nil
}

func (m *MockDownloader) ResumeTorrent(ctx context.Context, torrentID string) error {
	return nil
}

func (m *MockDownloader) RemoveTorrent(ctx context.Context, torrentID string, removeFiles bool) error {
	return nil
}

func (m *MockDownloader) GetTorrentInfo(ctx context.Context, torrentID string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"hash":   torrentID,
		"name":    "mock_torrent",
		"status":  "stopped",
		"size":    0,
		"progress": 0.0,
	}, nil
}

func (m *MockDownloader) Name() string {
	return "Mock"
}
