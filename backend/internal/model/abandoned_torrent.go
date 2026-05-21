package model

import "time"

// AbandonedTorrent 永久放弃的 InfoHash 黑名单。
//
// 用途：当一个 magnet 经长时间观察确认是死种（24h+ 拿不到元数据 / 0 做种 /
// scrape 探活回 0），qbit_sync 会把它从 qBit 和 download 表里硬删，同时写一行
// 到这里，后续 Orchestrator 在排序候选时会主动跳过这些 hash，避免反复重抓。
//
// 与早期靠 download.status=failed 当黑名单的做法相比：
//   - 不污染下载列表 UI（用户看到的"失败任务"不再是僵尸数据）
//   - 黑名单语义独立，可以记录原因 / 探活样本数等元数据
//   - 删除下载记录不会丢失黑名单状态
type AbandonedTorrent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InfoHash   string    `gorm:"uniqueIndex;not null" json:"info_hash"` // 大写 hex
	AnimeID    *uint     `gorm:"index" json:"anime_id"`                 // 首次入队时归属的 anime（仅供诊断）
	Title      string    `json:"title"`                                 // 原始种子标题（诊断用）
	Reason     string    `json:"reason"`                                // 拉黑原因
	AbandonedAt time.Time `gorm:"index" json:"abandoned_at"`
}

func (AbandonedTorrent) TableName() string { return "abandoned_torrent" }
