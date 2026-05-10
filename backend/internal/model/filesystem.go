package model

import "time"

// FileSystemEntry 文件系统条目
type FileSystemEntry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"index;not null" json:"name"`
	Path      string    `gorm:"index;not null" json:"path"`
	IsDir     bool      `gorm:"index;default:false" json:"is_dir"`
	Size      int64     `gorm:"default:0" json:"size"`
	ParentID *uint     `gorm:"index" json:"parent_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (FileSystemEntry) TableName() string { return "filesystem_entries" }
