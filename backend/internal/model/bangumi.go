package model

// BangumiAnime Bangumi 番剧数据
type BangumiAnime struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	NameCN     string  `json:"name_cn"`
	Summary    string  `json:"summary"`
	ImageURL   string  `json:"image"`
	Rating     float64 `json:"rating_score"`
	AirDate    string  `json:"air_date"`
	AirWeekday int     `json:"air_weekday"`
	EpsCount   int     `json:"eps_count"`
	Type       int     `json:"type"`

	// 详情字段（仅在详情接口返回）
	TotalEpisodes int              `json:"total_episodes,omitempty"`
	Platform      string           `json:"platform,omitempty"`
	Rank          int              `json:"rank,omitempty"`
	Tags          []string         `json:"tags,omitempty"`
	Infobox       []BangumiInfoKV  `json:"infobox,omitempty"`
}

// BangumiInfoKV infobox 键值对（值可能是 string 或 [{v: string}]）
type BangumiInfoKV struct {
	Key   string   `json:"key"`
	Value string   `json:"value"`
	Items []string `json:"items,omitempty"`
}

// BangumiCharacter 角色 + 声优
type BangumiCharacter struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	NameCN   string `json:"name_cn,omitempty"`
	Relation string `json:"relation"`
	ImageURL string `json:"image,omitempty"`
	Actor    string `json:"actor,omitempty"`
}

// InfoboxKV Bangumi infobox 条目
type InfoboxKV struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// BangumiCharacterDetail 角色详情（来自 /v0/characters/{id}）
type BangumiCharacterDetail struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	NameCN     string      `json:"name_cn,omitempty"`
	Type       int         `json:"type"`
	Summary    string      `json:"summary,omitempty"`
	Gender     string      `json:"gender,omitempty"`
	BloodType  int         `json:"blood_type,omitempty"`
	BirthYear  int         `json:"birth_year,omitempty"`
	BirthMonth int         `json:"birth_month,omitempty"`
	BirthDay   int         `json:"birth_day,omitempty"`
	Images     struct {
		Large  string `json:"large,omitempty"`
		Medium string `json:"medium,omitempty"`
		Small  string `json:"small,omitempty"`
		Grid   string `json:"grid,omitempty"`
	} `json:"images"`
	Infobox []InfoboxKV `json:"infobox,omitempty"`
	Stat    struct {
		Comments int `json:"comments"`
		Collects int `json:"collects"`
	} `json:"stat"`
}

// BangumiCalendarDay 日历中的一天
type BangumiCalendarDay struct {
	WeekdayID int            `json:"weekday_id"`
	WeekdayCN string         `json:"weekday_cn"`
	Items     []BangumiAnime `json:"items"`
}

// SubStatus holds subscription status for a Bangumi anime.
type SubStatus struct {
	IsSubscribed bool
	LocalID      uint
}

// BangumiAnimeWithStatus extends BangumiAnime with local subscription info.
type BangumiAnimeWithStatus struct {
	BangumiAnime
	IsSubscribed bool `json:"is_subscribed"`
	LocalID      uint `json:"local_id,omitempty"`
}

// BangumiCalendarDayWithStatus extends BangumiCalendarDay with subscription info.
type BangumiCalendarDayWithStatus struct {
	WeekdayID int                      `json:"weekday_id"`
	WeekdayCN string                   `json:"weekday_cn"`
	Items     []BangumiAnimeWithStatus `json:"items"`
}
