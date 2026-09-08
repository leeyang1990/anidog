package mock

import (
	"context"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/downloader"
)

type MockDownloader struct{}

func NewProvider(cfg *config.Config) (downloader.TorrentEngine, error) {
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
		"hash":     torrentID,
		"name":     "mock_torrent",
		"status":   "stopped",
		"size":     0,
		"progress": 0.0,
	}, nil
}

func (m *MockDownloader) Name() string {
	return "Mock"
}

func (m *MockDownloader) ListTorrents(context.Context) ([]downloader.TorrentSnapshot, error) {
	return nil, nil
}

func (m *MockDownloader) GetTotalWasted(context.Context, string) (int64, error) { return 0, nil }

func (m *MockDownloader) SetForceStart(context.Context, string, bool) error { return nil }

func (m *MockDownloader) SetPolicy(context.Context, downloader.TorrentPolicy) error { return nil }

func (m *MockDownloader) SetHTTPProxy(context.Context, string) error { return nil }

func (m *MockDownloader) Health(context.Context) downloader.EngineHealth {
	return downloader.EngineHealth{Online: true, Name: m.Name()}
}

func (m *MockDownloader) Close() error { return nil }

var _ downloader.TorrentEngine = (*MockDownloader)(nil)
