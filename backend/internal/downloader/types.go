package downloader

// 共享类型定义

// DownloadStatus 下载状态常量
const (
	StatusPending     = "pending"
	StatusDownloading = "downloading"
	StatusCompleted   = "completed"
	StatusPaused      = "paused"
	StatusFailed      = "error"
)
