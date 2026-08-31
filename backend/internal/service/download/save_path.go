package download

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
)

var reInvalidPath = regexp.MustCompile(`[\\/:*?"<>|]`)

const torrentRaceDir = ".anidog-race"

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

	title := SanitizeTitle(anime.MediaSeriesTitle())
	dir := title
	if year := anime.MediaSeriesYear(); year > 0 {
		dir = fmt.Sprintf("%s (%d)", title, year)
	}

	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	}

	return filepath.Join(mediaRoot, dir, fmt.Sprintf("Season %02d", season))
}

// BuildTorrentRaceSavePath gives every episode candidate a private directory.
// qBittorrent creates sparse/incomplete files as soon as a task starts; keeping
// them under a hidden staging root prevents Emby from treating every racing
// candidate as a separate playable episode.
func BuildTorrentRaceSavePath(mediaRoot string, animeID uint, episode int, infoHash, rawURL string) string {
	if mediaRoot == "" {
		return ""
	}
	key := strings.ToUpper(strings.TrimSpace(infoHash))
	if key == "" {
		sum := sha256.Sum256([]byte(rawURL))
		key = fmt.Sprintf("%x", sum[:8])
	}
	return filepath.Join(mediaRoot, torrentRaceDir,
		fmt.Sprintf("anime-%d", animeID),
		fmt.Sprintf("episode-%03d", episode),
		SanitizeTitle(key))
}

// BuildTorrentPackSavePath 给整季/合集候选独立目录。它仍放在 .anidog-race
// 下，下载完成前不会被 Emby 当成正式媒体扫描到。
func BuildTorrentPackSavePath(mediaRoot string, animeID uint, episodeStart, episodeEnd int, infoHash, rawURL string) string {
	if mediaRoot == "" {
		return ""
	}
	key := strings.ToUpper(strings.TrimSpace(infoHash))
	if key == "" {
		sum := sha256.Sum256([]byte(rawURL))
		key = fmt.Sprintf("%x", sum[:8])
	}
	return filepath.Join(mediaRoot, torrentRaceDir,
		fmt.Sprintf("anime-%d", animeID),
		fmt.Sprintf("season-pack-%03d-%03d", episodeStart, episodeEnd),
		SanitizeTitle(key))
}

func IsTorrentRaceSavePath(path string) bool {
	_, ok := TorrentRaceMediaRoot(path)
	return ok
}

// TorrentRaceMediaRoot returns the media library root preceding
// /.anidog-race/. It rejects lookalike names such as ".anidog-race-old".
func TorrentRaceMediaRoot(path string) (string, bool) {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(filepath.Separator))
	for i, part := range parts {
		if part != torrentRaceDir {
			continue
		}
		root := strings.Join(parts[:i], string(filepath.Separator))
		if filepath.IsAbs(clean) {
			root = string(filepath.Separator) + strings.TrimPrefix(root, string(filepath.Separator))
		}
		if root == "" {
			root = string(filepath.Separator)
		}
		return filepath.Clean(root), true
	}
	return "", false
}
