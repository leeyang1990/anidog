package downloader

import "context"

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

// ProviderConfig 下载器配置接口
type ProviderConfig interface {
	// Validate 验证配置有效性
	Validate() error

	// GetType 返回下载器类型
	GetType() string
}
