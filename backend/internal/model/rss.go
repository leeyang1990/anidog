package model

import "time"

// RSSFeed RSS订阅源数据库模型
type RSSFeed struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"index;not null" json:"name"`
	URL           string    `gorm:"uniqueIndex;not null" json:"url"`
	Description   *string   `json:"description"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	LastCheck     *time.Time `json:"last_check"`
	CheckInterval int       `gorm:"default:30" json:"check_interval"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Aggregate bool   `gorm:"default:false" json:"aggregate"`
	Parser    string `gorm:"default:'mikan'" json:"parser"`

	Rules []RSSRule `gorm:"foreignKey:RSSFeedID;constraint:OnDelete:CASCADE" json:"rules,omitempty"`
}

func (RSSFeed) TableName() string { return "rssfeed" }

// RSSRule RSS过滤规则数据库模型
type RSSRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Keyword   string    `gorm:"not null" json:"keyword"`
	IsRegex   bool      `gorm:"default:false" json:"is_regex"`
	Include   bool      `gorm:"default:true" json:"include"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	RSSFeedID uint      `gorm:"index;not null" json:"rss_feed_id"`
	RSSFeed   *RSSFeed  `gorm:"foreignKey:RSSFeedID" json:"-"`
}

func (RSSRule) TableName() string { return "rssrule" }

// RSSEntry RSS条目记录数据库模型
type RSSEntry struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	EntryID     string     `gorm:"index;not null" json:"entry_id"`
	Title       string     `gorm:"not null" json:"title"`
	Link        string     `gorm:"not null" json:"link"`
	Published   *time.Time `json:"published"`
	ProcessedAt time.Time  `json:"processed_at"`
	Downloaded  bool       `gorm:"default:false" json:"downloaded"`
	RSSFeedID   uint       `gorm:"index;not null" json:"rss_feed_id"`

	// 解析后字段（用于诊断/筛选）
	ParsedAnime   *string `json:"parsed_anime,omitempty"`
	ParsedEpisode *int    `json:"parsed_episode,omitempty"`
	ParsedGroup   *string `json:"parsed_group,omitempty"`
	MatchedAnimeID *uint  `gorm:"index" json:"matched_anime_id,omitempty"`
}

func (RSSEntry) TableName() string { return "rssentry" }

// RSSFeedCreate 创建RSS订阅源请求
type RSSFeedCreate struct {
	Name          string  `json:"name" binding:"required"`
	URL           string  `json:"url" binding:"required"`
	Description   *string `json:"description"`
	Enabled       *bool   `json:"enabled"`
	CheckInterval *int    `json:"check_interval"`
	Aggregate     *bool   `json:"aggregate"`
	Parser        *string `json:"parser"`
}

// RSSFeedUpdate 更新RSS订阅源请求
type RSSFeedUpdate struct {
	Name          *string `json:"name"`
	URL           *string `json:"url"`
	Description   *string `json:"description"`
	Enabled       *bool   `json:"enabled"`
	CheckInterval *int    `json:"check_interval"`
	Aggregate     *bool   `json:"aggregate"`
	Parser        *string `json:"parser"`
}

// RSSRuleCreate 创建RSS规则请求
type RSSRuleCreate struct {
	Name    string `json:"name" binding:"required"`
	Keyword string `json:"keyword" binding:"required"`
	IsRegex bool   `json:"is_regex"`
	Include bool   `json:"include"`
	Enabled bool   `json:"enabled"`
}

// RSSRuleUpdate 更新RSS规则请求
type RSSRuleUpdate struct {
	Name    *string `json:"name"`
	Keyword *string `json:"keyword"`
	IsRegex *bool   `json:"is_regex"`
	Include *bool   `json:"include"`
	Enabled *bool   `json:"enabled"`
}
