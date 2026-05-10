package stream

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

// seasonPatterns 从标题中提取季度的正则表达式
var seasonPatterns = []*regexp.Regexp{
	regexp.MustCompile(`第\s*([0-9一二三四五六七八九十]+)\s*季`),
	regexp.MustCompile(`(?i)Season\s*(\d+)`),
	regexp.MustCompile(`(?i)\bS(\d+)\b`),
	regexp.MustCompile(`\s+(\d+)(?:st|nd|rd|th)?\s*[Ss]eason\b`),
}

// chineseNumMap 中文数字映射
var chineseNumMap = map[string]int{
	"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
	"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
}

// detectSeason 从标题提取季度，默认返回 1
func detectSeason(title string) int {
	for _, pat := range seasonPatterns {
		if m := pat.FindStringSubmatch(title); len(m) > 1 {
			s := m[1]
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				return n
			}
			if n, ok := chineseNumMap[s]; ok {
				return n
			}
		}
	}
	return 1
}

// detectYear 获取首播年份
func detectYear(anime *model.Anime) int {
	if anime.Year != nil && *anime.Year > 1900 {
		return *anime.Year
	}
	return time.Now().Year()
}

// BuildMediaPath 按 Plex/Emby 规范生成输出路径。
//   - baseDir: /downloads
//   - anime: 番剧记录
//   - episodeNumber: 第几集（1-based）
//   - ext: 扩展名（如 .mp4）
//
// 返回：完整文件路径
// 例如: /downloads/葬送的芙莉莲 (2023)/Season 01/葬送的芙莉莲 S01E01.mp4
func BuildMediaPath(baseDir string, anime *model.Anime, episodeNumber int, ext string) string {
	title := sanitizeFileName(anime.Title)
	year := detectYear(anime)
	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	} else {
		season = detectSeason(anime.Title)
	}

	showDir := fmt.Sprintf("%s (%d)", title, year)
	seasonDir := fmt.Sprintf("Season %02d", season)
	fileName := fmt.Sprintf("%s S%02dE%02d%s", title, season, episodeNumber, ext)

	return filepath.Join(baseDir, showDir, seasonDir, fileName)
}

// buildLegacyPath 无 anime 信息时的回退路径: {base}/{anime_name}/{ep_name}{ext}
func buildLegacyPath(baseDir, animeName, episodeName, ext string) string {
	if animeName == "" {
		return filepath.Join(baseDir, sanitizeFileName(episodeName)+ext)
	}
	return filepath.Join(baseDir, sanitizeFileName(animeName), sanitizeFileName(episodeName)+ext)
}

// strings/filepath imports kept for future use
var _ = strings.ReplaceAll
