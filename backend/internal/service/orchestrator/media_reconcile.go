package orchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm/clause"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

const (
	MediaStateUninitialized = "uninitialized"
	MediaStateTracking      = "tracking"
	MediaStateFinalizing    = "finalizing"
	MediaStateArchived      = "archived"
	missingConfirmDelay     = 2 * time.Minute
	missingConfirmWindow    = 24 * time.Hour
)

type MediaAuditResult struct {
	Success   bool
	Confirmed bool
	Missing   []int
	Snapshot  string
}

var standardEpisodeFile = regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,3})`)

var mediaExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".ts": true,
	".m2ts": true, ".mov": true, ".webm": true, ".flv": true,
}

// reconcileMissingMedia checks completed episodes only inside the subscribed
// anime's current season directory. Missing media is converted to a transient
// failure so the same CheckAnime pass can schedule a replacement source.
func (o *Orchestrator) reconcileMissingMedia(ctx context.Context, anime *model.Anime) MediaAuditResult {
	if anime == nil {
		return MediaAuditResult{}
	}
	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	}
	seasonDir := dlservice.BuildAnimeSavePath(o.currentMediaRoot(ctx), anime)
	if err := o.checkMediaRootHealth(ctx, o.currentMediaRoot(ctx)); err != nil {
		zap.L().Error("媒体自愈：存储挂载不健康，禁止审计和补下载",
			zap.Uint("anime_id", anime.ID), zap.Error(err))
		return MediaAuditResult{}
	}
	present, mediaFileCount, scanErr := scanEpisodeFiles(seasonDir, season)
	if scanErr != nil && !os.IsNotExist(scanErr) {
		zap.L().Warn("媒体自愈：扫描当前季目录失败，不推进水位",
			zap.Uint("anime_id", anime.ID), zap.String("season_dir", seasonDir), zap.Error(scanErr))
		return MediaAuditResult{}
	}
	if _, err := os.Stat(seasonDir); err == nil && mediaFileCount > 0 && len(present) == 0 {
		// A batch torrent may use opaque filenames. Do not claim individual loss
		// unless at least one episode in this directory can be identified.
		zap.L().Warn("媒体自愈：当前季文件名无法识别集数，跳过以避免误补",
			zap.Uint("anime_id", anime.ID), zap.String("season_dir", seasonDir))
		return MediaAuditResult{}
	}

	var completed []model.Download
	if err := o.db.WithContext(ctx).
		Where("anime_id = ? AND episode_number IS NOT NULL AND status = ?", anime.ID, model.DownloadStatusCompleted).
		Find(&completed).Error; err != nil {
		zap.L().Warn("媒体自愈：查询完成任务失败", zap.Uint("anime_id", anime.ID), zap.Error(err))
		return MediaAuditResult{}
	}

	// Disk inventory is authoritative for files that can be identified. This
	// also adopts files placed there outside AniDog and prevents duplicates on
	// first subscription.
	for ep := range present {
		row := model.AnimeEpisode{AnimeID: anime.ID, EpisodeNumber: ep, Downloaded: true}
		_ = o.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "anime_id"}, {Name: "episode_number"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"downloaded": true}),
		}).Create(&row).Error
		_ = o.db.WithContext(ctx).Model(&model.Download{}).
			Where("anime_id = ? AND episode_number = ? AND media_missing = ?", anime.ID, ep, true).
			Updates(map[string]interface{}{"media_missing": false, "media_missing_at": nil}).Error
	}

	missingSet := make(map[int]bool)
	for _, dl := range completed {
		if dl.EpisodeNumber == nil || present[*dl.EpisodeNumber] {
			continue
		}
		missingSet[*dl.EpisodeNumber] = true
	}
	if len(missingSet) == 0 {
		return MediaAuditResult{Success: true, Confirmed: true}
	}

	now := time.Now()
	missing := make([]int, 0, len(missingSet))
	for ep := range missingSet {
		missing = append(missing, ep)
	}
	sort.Ints(missing)
	snapshotBytes, _ := json.Marshal(missing)
	snapshot := string(snapshotBytes)
	confirmed := anime.MediaMissingSnapshot == snapshot && anime.MediaMissingCheckedAt != nil &&
		now.Sub(*anime.MediaMissingCheckedAt) >= missingConfirmDelay &&
		now.Sub(*anime.MediaMissingCheckedAt) <= missingConfirmWindow
	if !confirmed {
		zap.L().Warn("媒体自愈：首次检测到文件缺失，等待二次确认",
			zap.Uint("anime_id", anime.ID), zap.String("season_dir", seasonDir), zap.Ints("episodes", missing))
		return MediaAuditResult{Success: true, Missing: missing, Snapshot: snapshot}
	}
	for _, ep := range missing {
		if err := o.db.WithContext(ctx).Model(&model.Download{}).
			Where("anime_id = ? AND episode_number = ? AND status = ?", anime.ID, ep, model.DownloadStatusCompleted).
			Updates(map[string]interface{}{"media_missing": true, "media_missing_at": &now}).Error; err != nil {
			zap.L().Warn("媒体自愈：标记媒体缺失失败", zap.Uint("anime_id", anime.ID), zap.Int("episode", ep), zap.Error(err))
			return MediaAuditResult{}
		}
		_ = o.db.WithContext(ctx).Model(&model.AnimeEpisode{}).
			Where("anime_id = ? AND episode_number = ?", anime.ID, ep).Update("downloaded", false).Error
	}
	zap.L().Warn("媒体自愈：检测到当前季文件缺失，将自动补全",
		zap.Uint("anime_id", anime.ID), zap.String("anime", anime.Title),
		zap.String("season_dir", seasonDir), zap.Ints("episodes", missing))
	return MediaAuditResult{Success: true, Confirmed: true, Missing: missing, Snapshot: snapshot}
}

func scanEpisodeFiles(seasonDir string, expectedSeason int) (map[int]bool, int, error) {
	present := make(map[int]bool)
	mediaFileCount := 0
	err := filepath.WalkDir(seasonDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !mediaExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
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
	return present, mediaFileCount, err
}

func (o *Orchestrator) persistMediaAuditResult(ctx context.Context, anime *model.Anime, state string, latestAired, expected int, result MediaAuditResult) {
	updates := map[string]interface{}{}
	if !result.Confirmed && len(result.Missing) > 0 {
		now := time.Now()
		updates["media_missing_snapshot"] = result.Snapshot
		updates["media_missing_checked_at"] = &now
	} else {
		updates["media_missing_snapshot"] = ""
		updates["media_missing_checked_at"] = nil
		updates["media_audit_episode"] = latestAired
		if state == MediaStateFinalizing && len(result.Missing) == 0 && latestAired >= expected {
			state = MediaStateArchived
		} else if state == MediaStateUninitialized {
			state = MediaStateTracking
		}
	}
	updates["media_management_state"] = state
	if err := o.db.WithContext(ctx).Model(&model.Anime{}).Where("id = ?", anime.ID).Updates(updates).Error; err != nil {
		zap.L().Warn("媒体自愈：保存审计状态失败", zap.Uint("anime_id", anime.ID), zap.Error(err))
		return
	}
	if value, ok := updates["media_audit_episode"].(int); ok {
		anime.MediaAuditEpisode = value
	}
	anime.MediaManagementState = state
	if value, ok := updates["media_missing_snapshot"].(string); ok {
		anime.MediaMissingSnapshot = value
	}
	if value, ok := updates["media_missing_checked_at"].(*time.Time); ok {
		anime.MediaMissingCheckedAt = value
	}
}

func (o *Orchestrator) checkMediaRootHealth(ctx context.Context, root string) error {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("媒体根目录不可用: %w", err)
	}
	if _, err := os.ReadDir(root); err != nil {
		return fmt.Errorf("媒体根目录不可读: %w", err)
	}
	requireRemote := false
	expectedType := ""
	if o.settingSvc != nil {
		if value, ok, _ := o.settingSvc.Get(ctx, "media.require_remote_mount"); ok {
			requireRemote = strings.EqualFold(strings.TrimSpace(value), "true")
		}
		if value, ok, _ := o.settingSvc.Get(ctx, "media.expected_mount_type"); ok {
			expectedType = strings.TrimSpace(value)
		}
	}
	fsType, err := mountedFilesystemType(root)
	if err != nil {
		return err
	}
	if expectedType != "" && fsType != expectedType {
		return fmt.Errorf("媒体挂载类型发生变化（期望 %s，当前 %s），可能已掉载", expectedType, fsType)
	}
	remote := false
	switch fsType {
	case "cifs", "nfs", "nfs4", "fuse.sshfs":
		remote = true
	}
	if requireRemote && !remote {
		return fmt.Errorf("媒体根目录不在远程挂载上（当前文件系统 %s）", fsType)
	}
	if expectedType == "" && remote && o.settingSvc != nil {
		if err := o.settingSvc.Set(ctx, "media.expected_mount_type", fsType); err != nil {
			return fmt.Errorf("保存媒体挂载基线失败: %w", err)
		}
	}
	probe, err := os.CreateTemp(root, ".anidog-mount-probe-")
	if err != nil {
		return fmt.Errorf("媒体根目录不可写: %w", err)
	}
	probeName := probe.Name()
	if _, err = probe.WriteString("ok"); err == nil {
		err = probe.Sync()
	}
	closeErr := probe.Close()
	removeErr := os.Remove(probeName)
	if err != nil {
		return fmt.Errorf("媒体挂载写入探针失败: %w", err)
	}
	if closeErr != nil || removeErr != nil {
		return fmt.Errorf("媒体挂载探针清理失败")
	}
	return nil
}

func mountedFilesystemType(path string) (string, error) {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", fmt.Errorf("读取挂载信息失败: %w", err)
	}
	defer file.Close()
	path = filepath.Clean(path)
	bestMount, bestType := "", ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " - ")
		if len(parts) != 2 {
			continue
		}
		left, right := strings.Fields(parts[0]), strings.Fields(parts[1])
		if len(left) < 5 || len(right) < 1 {
			continue
		}
		mountPoint := strings.ReplaceAll(left[4], `\040`, " ")
		if (path == mountPoint || strings.HasPrefix(path, strings.TrimSuffix(mountPoint, "/")+"/")) && len(mountPoint) > len(bestMount) {
			bestMount, bestType = mountPoint, right[0]
		}
	}
	if bestType == "" {
		return "", fmt.Errorf("找不到媒体根目录的挂载信息")
	}
	return bestType, scanner.Err()
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
