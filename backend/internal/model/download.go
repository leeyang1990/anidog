package model

import "time"// 下载状态常量
const (
	DownloadStatusPending     = "pending"
	DownloadStatusDownloading = "downloading"
	DownloadStatusCompleted   = "completed"
	DownloadStatusFailed      = "failed"
	DownloadStatusPaused      = "paused"
)

// 下载类型常量
const (
	DownloadTypeTorrent = "torrent"
	DownloadTypeStream  = "stream"
)

// Download 下载数据库模型
type Download struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TorrentID      string     `gorm:"uniqueIndex;not null" json:"torrent_id"`
	Name           string     `gorm:"index;not null" json:"name"`
	URL            string     `gorm:"not null" json:"url"`
	SavePath       *string    `json:"save_path"`
	Status         string     `gorm:"index;default:'pending'" json:"status"`
	Progress       float64    `gorm:"default:0" json:"progress"`
	DownloadedBytes *int64    `json:"downloaded_bytes"`
	TotalBytes     *int64     `json:"total_bytes"`
	DownloadSpeed  *int64     `json:"download_speed"`
	ETA            *int       `json:"eta"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at"`

	AnimeID       *uint  `gorm:"index" json:"anime_id"`
	EpisodeNumber *int   `json:"episode_number"`

	DownloadType  string `gorm:"index;default:'torrent'" json:"download_type"`
	StreamURL     *string `json:"stream_url"`
	StreamRuleID  *uint   `gorm:"index" json:"stream_rule_id"`
	StreamDetailURL *string `gorm:"index" json:"stream_detail_url"`
	StreamRoadName *string `gorm:"index" json:"stream_road_name"`
	FilePath      *string `json:"file_path"`
	Source        string  `gorm:"index;default:'manual'" json:"source"`

	// BT 专属：info_hash 用于匹配 qBit 中的种子，同步真实大小/进度
	InfoHash *string `gorm:"index" json:"info_hash"`
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

