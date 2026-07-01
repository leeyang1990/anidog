// Package orchestrator 统一调度 Stream / BT 两种下载源（按每集填坑）。
//
// 关于 RSS：RSS 是被动订阅 + 规则匹配的独立通道，由 RSSRefreshJob + 用户配置的
// rssrule 触发下载。orchestrator 不再把 RSS 作为"主动找资源"的一档，避免与
// BT 现场搜索重复。RSSEnabled 字段被保留，但语义改为"是否启用 RSS 定时刷新
// 与规则下载"，由 RSSRefreshJob 在每轮 Run 之前读取并判断是否跳过。
package orchestrator

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/service/setting"
)

// Preference 下载调度偏好（全局 + per-anime 合并后的有效值）。
type Preference struct {
	// 质量偏好
	Quality   string   // "720p" / "1080p" / "2160p" / ""
	Groups    []string // 字幕组白名单
	Languages []string // ["simplified","traditional","japanese","english"]
	MinSizeMB int
	MaxSizeMB int

	// 源启用
	StreamEnabled    bool
	BTEnabled        bool
	RSSEnabled       bool     // 仅控制 RSSRefreshJob 是否执行；orchestrator 不再读取此字段
	EnabledIndexers  []string // BT 启用的 indexers
	DisabledForAnime []string // 该 anime 关闭的 source_type

	// 调度
	Priority      []string // ["bt","stream"]（不再包含 rss）
	CheckInterval int      // 分钟
}

// IsSourceDisabled 判断某 source_type 对当前 anime 是否禁用。
// 注意：rss 不再是 orchestrator 的主动源，传 "rss" 永远视为禁用。
func (p Preference) IsSourceDisabled(srcType string) bool {
	for _, s := range p.DisabledForAnime {
		if s == srcType {
			return true
		}
	}
	switch srcType {
	case "stream":
		return !p.StreamEnabled
	case "bt":
		return !p.BTEnabled
	case "rss":
		return true
	}
	return true
}

// ToIndexerPref 转换为 indexer 包用的偏好结构
func (p Preference) ToIndexerPref() indexer.DownloadPreference {
	return indexer.DownloadPreference{
		Quality:   p.Quality,
		Groups:    append([]string{}, p.Groups...),
		Languages: append([]string{}, p.Languages...),
		MinSizeMB: p.MinSizeMB,
		MaxSizeMB: p.MaxSizeMB,
	}
}

// Defaults 默认全局偏好（用户没配置时）
func Defaults() Preference {
	return Preference{
		Quality:         "1080p",
		Groups:          []string{"LoliHouse", "桜都字幕组", "ANi", "喵萌奶茶屋", "北宇治字幕组"},
		Languages:       []string{"simplified"},
		MinSizeMB:       100,
		MaxSizeMB:       0, // 0 = 不限
		StreamEnabled:   true,
		BTEnabled:       true,
		RSSEnabled:      true,
		EnabledIndexers: []string{"mikan", "dmhy", "bangumimoe"},
		Priority:        []string{"bt", "stream"},
		CheckInterval:   30,
	}
}

// LoadGlobal 从 setting 表加载全局偏好，未设置的字段用 Defaults。
func LoadGlobal(ctx context.Context, svc *setting.Service) Preference {
	p := Defaults()
	if svc == nil {
		return p
	}

	get := func(key string) string {
		v, ok, _ := svc.Get(ctx, key)
		if !ok {
			return ""
		}
		return v
	}

	if v := get("download.quality"); v != "" {
		p.Quality = v
	}
	if v := get("download.groups"); v != "" {
		var arr []string
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			p.Groups = arr
		}
	}
	if v := get("download.languages"); v != "" {
		var arr []string
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			p.Languages = arr
		}
	}
	if v := get("download.min_size_mb"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.MinSizeMB = n
		}
	}
	if v := get("download.max_size_mb"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.MaxSizeMB = n
		}
	}

	// 源启用（默认 true）
	p.StreamEnabled = !isFalsy(get("download.source_enabled.stream"), true)
	p.BTEnabled = !isFalsy(get("download.source_enabled.bt"), true)
	p.RSSEnabled = !isFalsy(get("download.source_enabled.rss"), true)

	// Indexer 启用
	ixDefaults := map[string]bool{
		"mikan":      true,
		"dmhy":       true,
		"bangumimoe": true,
		"nyaa":       false,
	}
	var ixEnabled []string
	for name, defOn := range ixDefaults {
		key := "download.indexer_enabled." + name
		v := get(key)
		on := defOn
		if v == "true" || v == "1" {
			on = true
		} else if v == "false" || v == "0" {
			on = false
		}
		if on {
			ixEnabled = append(ixEnabled, name)
		}
	}
	if len(ixEnabled) > 0 {
		p.EnabledIndexers = ixEnabled
	}

	// 优先级（旧设置可能含 "rss"，过滤掉）
	if v := get("download.priority"); v != "" {
		var arr []string
		if err := json.Unmarshal([]byte(v), &arr); err == nil && len(arr) > 0 {
			filtered := arr[:0]
			for _, s := range arr {
				if s != "rss" {
					filtered = append(filtered, s)
				}
			}
			if len(filtered) > 0 {
				p.Priority = filtered
			}
		}
	}

	// 检查间隔
	if v := get("download.check_interval"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 5 {
			p.CheckInterval = n
		}
	}

	return p
}

// MergeWithAnime 把全局偏好与 anime.Override* 字段合并。
// 非空的 Override 覆盖全局；DisabledSources 追加到 DisabledForAnime。
func MergeWithAnime(global Preference, anime *model.Anime) Preference {
	out := global

	if anime == nil {
		return out
	}

	if anime.OverrideQuality != nil && *anime.OverrideQuality != "" {
		out.Quality = *anime.OverrideQuality
	}
	if anime.OverrideGroups != nil && *anime.OverrideGroups != "" {
		var arr []string
		if err := json.Unmarshal([]byte(*anime.OverrideGroups), &arr); err == nil && len(arr) > 0 {
			out.Groups = arr
		}
	}
	if anime.OverrideLanguages != nil && *anime.OverrideLanguages != "" {
		var arr []string
		if err := json.Unmarshal([]byte(*anime.OverrideLanguages), &arr); err == nil && len(arr) > 0 {
			out.Languages = arr
		}
	}
	if anime.DisabledSources != nil && *anime.DisabledSources != "" {
		var arr []string
		if err := json.Unmarshal([]byte(*anime.DisabledSources), &arr); err == nil {
			out.DisabledForAnime = arr
		}
	}

	return out
}

// isFalsy 返回 val 是否明确为 false；defaultTrue 指示为空时返回 false（不为 falsy）
// 即 "空 + defaultTrue" 情况下返回 false（表示"非 falsy"，即启用）
func isFalsy(val string, defaultTrue bool) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "" {
		return !defaultTrue
	}
	return v == "false" || v == "0" || v == "off" || v == "no"
}
