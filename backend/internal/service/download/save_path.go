package download

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
)

var reInvalidPath = regexp.MustCompile(`[\\/:*?"<>|]`)

// SanitizeTitle 去掉文件/目录名里的非法字符
func SanitizeTitle(s string) string {
	s = reInvalidPath.ReplaceAllString(s, "_")
	s = strings.TrimSpace(s)
	if s == "" {
		return "未知"
	}
	return s
}

// BuildAnimeSavePath 按 Plex/Emby 约定生成番剧下载目录。
// 示例: /downloads/葬送的芙莉莲 (2023)/Season 01
// mediaRoot 空返回空字符串（让下载器用默认目录）
func BuildAnimeSavePath(mediaRoot string, anime *model.Anime) string {
	if mediaRoot == "" {
		return ""
	}
	if anime == nil {
		return mediaRoot
	}

	title := SanitizeTitle(anime.Title)
	dir := title
	if anime.Year != nil && *anime.Year > 0 {
		dir = fmt.Sprintf("%s (%d)", title, *anime.Year)
	}

	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	}

	return filepath.Join(mediaRoot, dir, fmt.Sprintf("Season %02d", season))
}
