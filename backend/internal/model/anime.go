package model

import "time"

// AnimeStatus 番剧状态枚举
const (
	AnimeStatusOngoing = "ongoing"
	AnimeStatusFinished = "finished"
	AnimeStatusUpcoming = "upcoming"
	AnimeStatusUnknown  = "unknown"
)

// Anime 番剧数据库模型
type Anime struct {
	ID              uint            `gorm:"primaryKey" json:"id"`
	Title           string          `gorm:"index;not null" json:"title"`
	OriginalTitle   *string         `gorm:"index" json:"original_title"`
	Aliases         *string         `json:"aliases"`
	Description     *string         `json:"description"`
	Status          string          `gorm:"index;default:'unknown'" json:"status"`
	Season          *int            `json:"season"`
	Year            *int            `json:"year"`
	CoverURL        *string         `json:"cover_url"`
	EpisodeCount    *int            `json:"episode_count"`
	CurrentEpisode  *int            `json:"current_episode"`
	Directory       *string         `json:"directory"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`

	// 解析相关字段
	OfficialTitle *string `json:"official_title"`
	TitleRaw      *string `json:"title_raw"`
	SeasonRaw     *string `json:"season_raw"`
	GroupName     *string `json:"group_name"`
	DPI           *string `json:"dpi"`
	Source        *string `json:"source"`
	Subtitle      *string `json:"subtitle"`

	// 下载管理字段
	EpsCollect    bool    `gorm:"default:false" json:"eps_collect"`
	EpisodeOffset int     `gorm:"default:0" json:"episode_offset"`
	SeasonOffset  int     `gorm:"default:0" json:"season_offset"`
	Filter        *string `gorm:"column:filter" json:"filter"`
	RSSLink       *string `json:"rss_link"`
	RuleName      *string `json:"rule_name"`
	Added         bool    `gorm:"default:false" json:"added"`

	// 放送信息
	AirWeekday *int `json:"air_weekday"`

	// Bangumi 集成
	BangumiID     *int     `gorm:"index" json:"bangumi_id"`
	BangumiRating *float64 `json:"bangumi_rating"`
	IsSubscribed  bool     `gorm:"index;default:false" json:"is_subscribed"`

	// Mikan Project 番剧 ID（与 BangumiID 不同体系）
	// 订阅时由 Mikan Search 反查后写入；之后 BT indexer 走 Mikan RSS（按 bangumiId 推送）
	// 而非低召回率的关键词搜索。空 = 未反查 / 反查失败，indexer 退回搜索。
	MikanBangumiID *int `gorm:"index" json:"mikan_bangumi_id"`

	// 流媒体源偏好
	StreamRuleID    *uint   `json:"stream_rule_id" gorm:"index"`
	StreamDetailURL *string `json:"stream_detail_url"`
	StreamRoadName  *string `json:"stream_road_name"`
	StreamRuleName  *string `json:"stream_rule_name"`

	// 源健康状态（由 SourceHealthJob 定期更新）
	// 取值: "", "healthy", "degraded", "broken"
	// "": 未检测  healthy: 近期下载成功率高
	// degraded: 有若干失败  broken: 几乎全失败/检测到伪装流
	SourceHealthStatus *string    `json:"source_health_status" gorm:"index"`
	SourceHealthNote   *string    `json:"source_health_note"`
	SourceHealthAt     *time.Time `json:"source_health_at"`

	// 来源入口（信息展示用，不影响调度）
	// "bangumi" / "bt_search" / "manual"
	SourceOrigin string `json:"source_origin" gorm:"default:''"`

	// per-anime 偏好覆盖（nil/空 = 继承全局）
	OverrideQuality   *string `json:"override_quality"`
	OverrideGroups    *string `json:"override_groups" gorm:"type:text"`    // JSON array
	OverrideLanguages *string `json:"override_languages" gorm:"type:text"` // JSON array
	DisabledSources   *string `json:"disabled_sources" gorm:"type:text"`   // JSON array: ["stream","bt","rss"]

	// 关联
	Episodes []AnimeEpisode `gorm:"foreignKey:AnimeID" json:"episodes,omitempty"`
}

func (Anime) TableName() string { return "anime" }

// AnimeEpisode 番剧集数数据库模型
type AnimeEpisode struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	EpisodeNumber int       `gorm:"index;not null;uniqueIndex:idx_animeepisode_anime_ep,priority:2" json:"episode_number"`
	Title         *string   `json:"title"`
	NameCN        *string   `json:"name_cn"`
	AirDate       *string   `gorm:"index" json:"air_date"` // YYYY-MM-DD（来自 Bangumi /v0/episodes）
	FilePath      *string   `json:"file_path"`
	FileSize      *int64    `json:"file_size"`
	Downloaded    bool      `gorm:"index;default:false" json:"downloaded"`
	DownloadID    *string   `json:"download_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	AnimeID       uint      `gorm:"index;not null;uniqueIndex:idx_animeepisode_anime_ep,priority:1" json:"anime_id"`
	Anime         *Anime    `gorm:"foreignKey:AnimeID" json:"-"`
}

func (AnimeEpisode) TableName() string { return "animeepisode" }

// AnimeCreate 创建番剧请求
type AnimeCreate struct {
	Title           string   `json:"title" binding:"required"`
	OriginalTitle   *string  `json:"original_title"`
	Aliases         *string  `json:"aliases"`
	Description     *string  `json:"description"`
	Status          string   `json:"status"`
	Season          *int     `json:"season"`
	Year            *int     `json:"year"`
	CoverURL        *string  `json:"cover_url"`
	EpisodeCount    *int     `json:"episode_count"`
	Directory       *string  `json:"directory"`
	OfficialTitle   *string  `json:"official_title"`
	TitleRaw        *string  `json:"title_raw"`
	SeasonRaw       *string  `json:"season_raw"`
	GroupName       *string  `json:"group_name"`
	DPI             *string  `json:"dpi"`
	Source          *string  `json:"source"`
	Subtitle        *string  `json:"subtitle"`
	EpsCollect      bool     `json:"eps_collect"`
	EpisodeOffset   int      `json:"episode_offset"`
	SeasonOffset    int      `json:"season_offset"`
	Filter          *string  `json:"filter"`
	RSSLink         *string  `json:"rss_link"`
	AirWeekday      *int     `json:"air_weekday"`
	BangumiID       *int     `json:"bangumi_id"`
	BangumiRating   *float64 `json:"bangumi_rating"`
	IsSubscribed    bool     `json:"is_subscribed"`
}

