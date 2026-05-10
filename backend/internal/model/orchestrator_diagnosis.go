package model

import "time"

// OrchestratorDiagnosis Orchestrator 每次检查某集的诊断记录。
// 用于 UI 展示"第 X 集为什么还没下"的详细原因。
type OrchestratorDiagnosis struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	AnimeID       uint      `gorm:"index;not null" json:"anime_id"`
	EpisodeNumber int       `gorm:"index;not null" json:"episode_number"`
	SourceType    string    `gorm:"index;not null" json:"source_type"` // "stream"/"bt"/"rss"
	CheckedAt     time.Time `gorm:"index" json:"checked_at"`
	ResultCount   int       `json:"result_count"`             // 源返回多少条
	RankedOut     int       `json:"ranked_out"`               // 被偏好过滤掉多少
	Reason        string    `gorm:"type:text" json:"reason"`  // 拼接的失败原因
	BestTitle     string    `json:"best_title,omitempty"`     // 最佳候选标题（若有）
	BestScore     float64   `json:"best_score,omitempty"`
}

func (OrchestratorDiagnosis) TableName() string { return "orchestrator_diagnosis" }
