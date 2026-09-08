package downloader

import "context"

// TorrentSnapshot is the downloader-independent view consumed by AniDog's
// health patrol and UI. Provider-specific response maps must not escape the
// provider package.
type TorrentSnapshot struct {
	ID             string
	Name           string
	State          string
	HasMetadata    bool
	Size           int64
	Downloaded     int64
	DownloadSpeed  int64
	Progress       float64
	Availability   float64
	ConnectedSeeds int
	ETASeconds     int
	TotalWasted    int64
	ContentPath    string
}

// TorrentPolicy contains the settings that can be changed at runtime from the
// download settings page. Rates are bytes per second; zero means unlimited.
type TorrentPolicy struct {
	MaxActiveDownloads int
	MaxActiveTorrents  int
	DownloadRate       int64
	UploadRate         int64
}

// EngineHealth is returned by the system information endpoint.
type EngineHealth struct {
	Online       bool   `json:"online"`
	Name         string `json:"name"`
	Version      string `json:"version,omitempty"`
	ListenPort   int    `json:"listen_port,omitempty"`
	TorrentCount int    `json:"torrent_count"`
}

// Downloader 下载器统一接口
type Downloader interface {
	// AddTorrent 添加种子任务
	AddTorrent(ctx context.Context, torrentURL, savePath string) (string, error)

	// PauseTorrent 暂停下载
	PauseTorrent(ctx context.Context, torrentID string) error

	// ResumeTorrent 恢复下载
	ResumeTorrent(ctx context.Context, torrentID string) error

	// RemoveTorrent 删除任务
	RemoveTorrent(ctx context.Context, torrentID string, removeFiles bool) error

	// GetTorrentInfo 获取任务信息
	GetTorrentInfo(ctx context.Context, torrentID string) (map[string]interface{}, error)

	// Name 返回下载器名称
	Name() string
}

// TorrentEngine is the complete runtime boundary used by AniDog. Both the
// embedded Go engine and the optional qBittorrent compatibility provider
// implement this interface, so scheduling, health checks and settings do not
// depend on a specific downloader process.
type TorrentEngine interface {
	Downloader
	ListTorrents(ctx context.Context) ([]TorrentSnapshot, error)
	GetTotalWasted(ctx context.Context, torrentID string) (int64, error)
	SetForceStart(ctx context.Context, torrentID string, enabled bool) error
	SetPolicy(ctx context.Context, policy TorrentPolicy) error
	SetHTTPProxy(ctx context.Context, proxyURL string) error
	Health(ctx context.Context) EngineHealth
	Close() error
}

// ProviderConfig 下载器配置接口
type ProviderConfig interface {
	// Validate 验证配置有效性
	Validate() error

	// GetType 返回下载器类型
	GetType() string
}
