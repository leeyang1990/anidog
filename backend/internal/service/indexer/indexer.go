// Package indexer 抽象 BT 种子源的搜索与聚合。
// 每个 Indexer 实现 Search，返回原始候选；Aggregator 负责并发调用、去重、解析标题、按偏好评分。
package indexer

import (
	"context"
	"time"

	"github.com/anidog/anidog-go/internal/service/titleparse"
)

// Indexer 是 BT 种子源的统一接口。
type Indexer interface {
	// Name 返回唯一标识，如 "mikan"、"dmhy"。
	Name() string

	// Search 按关键词搜索，返回候选种子（不含已解析字段）。
	// 实现应在合理超时内返回，网络错误可直接返回 err（由 Aggregator 聚合处理）。
	Search(ctx context.Context, keyword string) ([]Candidate, error)
}

// Candidate 单条候选种子。
type Candidate struct {
	Title      string                   `json:"title"`       // 原始标题
	Parsed     *titleparse.ParsedTitle  `json:"parsed"`      // 由 Aggregator 填充
	MagnetURL  string                   `json:"magnet_url"`  // 优先 magnet
	TorrentURL string                   `json:"torrent_url"` // 直链 .torrent（备用）
	InfoHash   string                   `json:"info_hash"`   // 去重用（可能从 magnet 解析）
	PubDate    time.Time                `json:"pub_date"`
	Size       int64                    `json:"size"`  // 字节
	Seeders    int                      `json:"seeders"`
	Leechers   int                      `json:"leechers"`
	SourceName string                   `json:"source_name"` // indexer name
	DetailURL  string                   `json:"detail_url"`  // 种子详情页，给用户跳转
}

// ScoredCandidate 评分后的候选。
type ScoredCandidate struct {
	Candidate
	Score  float64  `json:"score"`
	Reason []string `json:"reason,omitempty"` // 评分理由（调试 + 诊断用）
}

// DownloadPreference 用户的下载偏好，Aggregator 据此评分。
type DownloadPreference struct {
	Quality   string   // "1080p" / "720p" / "2160p" / ""（不限）
	Groups    []string // 字幕组白名单
	Languages []string // ["simplified","traditional","japanese","english"]
	MinSizeMB int      // 0 = 不限
	MaxSizeMB int      // 0 = 不限
}
