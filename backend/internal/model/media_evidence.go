package model

import "time"

// MediaEvidence is a content fact, independent of source and user preferences.
// Only a complete local video digest can address this cache. Remote frame
// matches can refer to it, but must never claim a complete video digest.
type MediaEvidence struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	VideoHash        string    `gorm:"uniqueIndex:media_evidence_content;not null" json:"video_hash"`
	InspectorVersion string    `gorm:"uniqueIndex:media_evidence_content;not null" json:"inspector_version"`
	Evidence         string    `gorm:"type:text;not null" json:"evidence"`
	OriginDownloadID *uint     `json:"origin_download_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (MediaEvidence) TableName() string { return "media_evidence" }
