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
