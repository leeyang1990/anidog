package model

import "time"

// NotificationChannel 通知渠道数据库模型
type NotificationChannel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"not null" json:"type"` // telegram/bark/webhook/discord/server_chan/wecom
	Name      string    `gorm:"not null" json:"name"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Config    string    `gorm:"not null;type:text" json:"config"` // JSON 配置
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (NotificationChannel) TableName() string { return "notificationchannel" }

// EpisodeNotification 单集通知去重记录。
//
// 设计目标：让 "<番剧> S<NN>E<MM> 已更新" 这种通知严格按 (anime_id, season, episode)
// 维度去重，无论触发路径是 BT 完成、Stream 备援完成、qbit_sync 误标 completed、
// 还是 dl 行被删后又新建一个新行 —— 通知都只发一次。
//
// 用法：notifyCompletion 入口先 INSERT 这条记录，依赖 (anime_id, episode_number) 唯一约束；
// 唯一冲突 → 已发过 → return；插入成功 → 才执行 Broadcast。
//
// 注意：这是"成功推送过的事实"，不要随业务删除（即使 download 行被 abandonDeadTorrent
// 删除，对应集的通知记录仍要保留，否则下一轮 Stream 接手完成又会重发）。
type EpisodeNotification struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	AnimeID       uint      `gorm:"not null;uniqueIndex:idx_epnotif_anime_ep,priority:1" json:"anime_id"`
	EpisodeNumber int       `gorm:"not null;uniqueIndex:idx_epnotif_anime_ep,priority:2" json:"episode_number"`
	Season        int       `json:"season"`
	NotifiedAt    time.Time `gorm:"not null" json:"notified_at"`
}

func (EpisodeNotification) TableName() string { return "episode_notification" }

// NotificationChannelCreate 创建通知渠道请求
type NotificationChannelCreate struct {
	Type    string `json:"type" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Enabled *bool  `json:"enabled"`
	Config  string `json:"config" binding:"required"`
}

// NotificationChannelUpdate 更新通知渠道请求
type NotificationChannelUpdate struct {
	Type    *string `json:"type"`
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
	Config  *string `json:"config"`
}
