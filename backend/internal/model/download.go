package model

import "time" // 下载状态常量
const (
	DownloadStatusPending     = "pending"
	DownloadStatusDownloading = "downloading"
	DownloadStatusCompleted   = "completed"
	DownloadStatusFailed      = "failed"
	DownloadStatusPaused      = "paused"
)

// 失败种类（FailureKind）—— 用来区分"可重试"与"永久失败"。
//   - transient：临时故障，由 RetryFailedJob 按退避节奏自动重试
//     例：流媒体 m3u8 签名链接过期（IO error: End of file / 403）
//     BT 死种：DHT 短期找不到 peer（meta_dl 超时）—— 等下一轮新 hash
//   - permanent：永久失败，不再重试
//     例：磁盘满、文件 IO 错、格式不支持
//   - "" （空）：尚未分类（旧记录或代码未覆盖路径）—— 走原逻辑
const (
	FailureKindTransient = "transient"
	FailureKindPermanent = "permanent"
	// exhausted：快速重试预算已耗尽，但外部下载源未来可能恢复。
	// 不进入 5 分钟 RetryJob，由 30 分钟 Orchestrator 在长冷却后半开探测。
	FailureKindExhausted = "exhausted"
)

// 下载类型常量
const (
	DownloadTypeTorrent = "torrent"
	DownloadTypeStream  = "stream"
)

// Download 下载数据库模型
type Download struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	TorrentID       string     `gorm:"uniqueIndex;not null" json:"torrent_id"`
	Name            string     `gorm:"index;not null" json:"name"`
	URL             string     `gorm:"not null" json:"url"`
	SavePath        *string    `json:"save_path"`
	Status          string     `gorm:"index;default:'pending'" json:"status"`
	Progress        float64    `gorm:"default:0" json:"progress"`
	DownloadedBytes *int64     `json:"downloaded_bytes"`
	TotalBytes      *int64     `json:"total_bytes"`
	DownloadSpeed   *int64     `json:"download_speed"`
	ETA             *int       `json:"eta"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CompletedAt     *time.Time `json:"completed_at"`

	AnimeID       *uint `gorm:"index" json:"anime_id"`
	EpisodeNumber *int  `json:"episode_number"`

	DownloadType    string  `gorm:"index;default:'torrent'" json:"download_type"`
	StreamURL       *string `json:"stream_url"`
	StreamRuleID    *uint   `gorm:"index" json:"stream_rule_id"`
	StreamDetailURL *string `gorm:"index" json:"stream_detail_url"`
	StreamRoadName  *string `gorm:"index" json:"stream_road_name"`
	FilePath        *string `json:"file_path"`
	Source          string  `gorm:"index;default:'manual'" json:"source"`

	// BT 专属：info_hash 用于匹配 qBit 中的种子，同步真实大小/进度
	InfoHash *string `gorm:"index" json:"info_hash"`

	// 失败重试相关（参见 FailureKindTransient/Permanent）。
	// RetryCount：已经尝试重试的次数（不含首次执行）。
	// LastError：最近一次失败的 stderr 摘要或错误消息，给前端展示。
	// FailureKind：'transient' 可重试 / 'permanent' 放弃 / ''（旧逻辑）。
	// NextRetryAt：下一次允许重试的最早时间；RetryFailedJob 会扫这个字段。
	RetryCount  int        `gorm:"default:0" json:"retry_count"`
	LastError   string     `gorm:"type:text" json:"last_error"`
	FailureKind string     `gorm:"index;type:varchar(20)" json:"failure_kind"`
	NextRetryAt *time.Time `gorm:"index" json:"next_retry_at"`
}

func (Download) TableName() string { return "download" }

// DownloadCreate 创建下载请求
type DownloadCreate struct {
	Name          string  `json:"name"`
	URL           string  `json:"url" binding:"required"`
	SavePath      *string `json:"save_path"`
	DownloadType  string  `json:"download_type"`
	StreamRuleID  *uint   `json:"stream_rule_id"`
	AnimeID       *uint   `json:"anime_id"`
	EpisodeNumber *int    `json:"episode_number"`
	// Source 覆盖，默认 "manual"；合法值: "manual"/"bt"/"stream"/"rss"
	Source *string `json:"source"`
	// 前端兼容字段
	MagnetLink *string `json:"magnet_link"`
	Title      *string `json:"title"`
}
