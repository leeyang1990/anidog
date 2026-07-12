package orchestrator

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

var standardEpisodeFile = regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,3})`)

var mediaExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".ts": true,
	".m2ts": true, ".mov": true, ".webm": true, ".flv": true,
}

// reconcileMissingMedia checks completed episodes only inside the subscribed
// anime's current season directory. Missing media is converted to a transient
// failure so the same CheckAnime pass can schedule a replacement source.
func (o *Orchestrator) reconcileMissingMedia(ctx context.Context, anime *model.Anime) []int {
	if anime == nil {
		return nil
	}
	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	}
	seasonDir := dlservice.BuildAnimeSavePath(o.currentMediaRoot(ctx), anime)
	present, mediaFileCount := scanEpisodeFiles(seasonDir, season)
	if _, err := os.Stat(seasonDir); err == nil && mediaFileCount > 0 && len(present) == 0 {
		// A batch torrent may use opaque filenames. Do not claim individual loss
		// unless at least one episode in this directory can be identified.
		zap.L().Warn("媒体自愈：当前季文件名无法识别集数，跳过以避免误补",
			zap.Uint("anime_id", anime.ID), zap.String("season_dir", seasonDir))
		return nil
	}

	var completed []model.Download
	if err := o.db.WithContext(ctx).
		Where("anime_id = ? AND episode_number IS NOT NULL AND status = ?", anime.ID, model.DownloadStatusCompleted).
		Find(&completed).Error; err != nil {
		zap.L().Warn("媒体自愈：查询完成任务失败", zap.Uint("anime_id", anime.ID), zap.Error(err))
		return nil
	}

	missingSet := make(map[int]bool)
	for _, dl := range completed {
		if dl.EpisodeNumber == nil || present[*dl.EpisodeNumber] {
			continue
		}
		missingSet[*dl.EpisodeNumber] = true
	}
	if len(missingSet) == 0 {
		return nil
	}

	now := time.Now()
	missing := make([]int, 0, len(missingSet))
	for ep := range missingSet {
		missing = append(missing, ep)
		result := o.db.WithContext(ctx).Model(&model.Download{}).
			Where("anime_id = ? AND episode_number = ? AND status = ?", anime.ID, ep, model.DownloadStatusCompleted).
			Updates(map[string]interface{}{
				"status":        model.DownloadStatusFailed,
				"failure_kind":  "transient",
				"last_error":    "媒体文件已从当前季目录中丢失，等待自动补全",
				"next_retry_at": &now,
			})
		if result.Error != nil {
			zap.L().Warn("媒体自愈：标记缺失集失败", zap.Uint("anime_id", anime.ID), zap.Int("episode", ep), zap.Error(result.Error))
		}
	}
	sort.Ints(missing)
	zap.L().Warn("媒体自愈：检测到当前季文件缺失，将自动补全",
		zap.Uint("anime_id", anime.ID), zap.String("anime", anime.Title),
		zap.String("season_dir", seasonDir), zap.Ints("episodes", missing))
	return missing
}

func scanEpisodeFiles(seasonDir string, expectedSeason int) (map[int]bool, int) {
	present := make(map[int]bool)
	mediaFileCount := 0
	_ = filepath.WalkDir(seasonDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !mediaExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			return nil
		}
		mediaFileCount++
		name := entry.Name()
		if match := standardEpisodeFile.FindStringSubmatch(name); len(match) == 3 {
			season := parsePositiveInt(match[1])
			episode := parsePositiveInt(match[2])
			if episode > 0 && (season == 0 || season == expectedSeason) {
				present[episode] = true
			}
			return nil
		}
		parsed := titleparse.Parse(name)
		if parsed.SeasonNum != nil && *parsed.SeasonNum != expectedSeason {
			return nil
		}
		if parsed.EpisodeNum != nil && *parsed.EpisodeNum > 0 {
			present[*parsed.EpisodeNum] = true
		}
		return nil
	})
	return present, mediaFileCount
}

func parsePositiveInt(value string) int {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
