// Package mediapolicy resolves the same media requirements for discovery,
// preflight, final acceptance and library auditing.
package mediapolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anidog/anidog-go/internal/mediaquality"
	"github.com/anidog/anidog-go/internal/model"
	"gorm.io/gorm"
)

type Goal struct {
	Quality       string
	Languages     []string
	AllowFallback bool
}

func Default() Goal { return Goal{Quality: "1080p", Languages: []string{"simplified"}} }

func Resolve(values map[string]string, anime *model.Anime) (Goal, error) {
	g := Default()
	if v := strings.TrimSpace(values["download.quality"]); v != "" {
		g.Quality = v
	}
	if v := strings.TrimSpace(values["download.languages"]); v != "" {
		languages, err := parseLanguages(v)
		if err != nil {
			return Goal{}, fmt.Errorf("download.languages: %w", err)
		}
		g.Languages = languages // An explicit global [] means unrestricted.
	}
	switch v := strings.TrimSpace(values["download.allow_preference_fallback"]); v {
	case "", "false", "0":
	case "true", "1":
		g.AllowFallback = true
	default:
		return Goal{}, fmt.Errorf("download.allow_preference_fallback 无效")
	}
	return Apply(g, anime)
}

func Apply(g Goal, anime *model.Anime) (Goal, error) {
	g.Languages = append([]string{}, g.Languages...)
	if anime != nil {
		if anime.OverrideQuality != nil && strings.TrimSpace(*anime.OverrideQuality) != "" {
			g.Quality = strings.TrimSpace(*anime.OverrideQuality)
		}
		if anime.OverrideLanguages != nil && strings.TrimSpace(*anime.OverrideLanguages) != "" {
			languages, err := parseLanguages(*anime.OverrideLanguages)
			if err != nil {
				return Goal{}, fmt.Errorf("番剧字幕偏好无效: %w", err)
			}
			// Per-anime empty selection means inherit, matching the settings UI.
			if len(languages) > 0 {
				g.Languages = languages
			}
		}
	}
	if g.Quality != "" && mediaquality.Height(g.Quality) == 0 {
		return Goal{}, fmt.Errorf("画质偏好无效: %s", g.Quality)
	}
	return g, nil
}

func parseLanguages(raw string) ([]string, error) {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("字幕偏好必须是数组: %w", err)
	}
	if values == nil {
		return nil, fmt.Errorf("字幕偏好不能为 null")
	}
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, language := range values {
		switch language {
		case "simplified", "traditional", "english", "japanese":
		default:
			return nil, fmt.Errorf("未知字幕语言: %s", language)
		}
		if !seen[language] {
			seen[language] = true
			result = append(result, language)
		}
	}
	return result, nil
}

func Load(ctx context.Context, db *gorm.DB, animeID *uint) (Goal, error) {
	var rows []model.Setting
	if err := db.WithContext(ctx).Where("key IN ?", []string{"download.quality", "download.languages", "download.allow_preference_fallback"}).Find(&rows).Error; err != nil {
		return Goal{}, err
	}
	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	var anime *model.Anime
	if animeID != nil {
		anime = &model.Anime{}
		if err := db.WithContext(ctx).First(anime, *animeID).Error; err != nil {
			return Goal{}, err
		}
	}
	return Resolve(values, anime)
}
