package download

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/shirou/gopsutil/v3/disk"
)

const gib = int64(1024 * 1024 * 1024)

type capacityProbe func(string) (*disk.UsageStat, error)

func capacityTaskFromDownload(row *model.Download, remaining bool) *Task {
	t := &Task{DownloadType: row.DownloadType, Scope: row.Scope, EpisodeStart: row.EpisodeStart, EpisodeEnd: row.EpisodeEnd}
	if row.SavePath != nil {
		t.SavePath = *row.SavePath
	}
	if row.TotalBytes != nil {
		t.EstimatedBytes = *row.TotalBytes
	}
	t.EstimatedBytes = estimatedTaskBytes(t)
	if remaining && row.DownloadedBytes != nil {
		t.EstimatedBytes -= *row.DownloadedBytes
		if t.EstimatedBytes <= 0 {
			t.EstimatedBytes = 1
		}
	}
	return t
}

func estimatedTaskBytes(task *Task) int64 {
	if task == nil {
		return 0
	}
	if task.EstimatedBytes > 0 {
		return task.EstimatedBytes
	}
	perEpisode := 2 * gib
	if task.DownloadType == model.DownloadTypeTorrent {
		perEpisode = 4 * gib
	}
	count := int64(1)
	if task.Scope == model.DownloadScopeSeason && task.EpisodeStart != nil && task.EpisodeEnd != nil && *task.EpisodeEnd >= *task.EpisodeStart {
		count = int64(*task.EpisodeEnd - *task.EpisodeStart + 1)
	}
	if count > math.MaxInt64/perEpisode {
		return math.MaxInt64
	}
	return count * perEpisode
}

func withinStorage(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func existingStoragePath(path string) string {
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}

func (s *Service) ensureMediaCapacity(ctx context.Context) error {
	return s.ensureTaskCapacity(ctx, nil, 0)
}

// Percent usage is informational: on a large shared NAS, 95% can still leave
// terabytes. Admission depends on available bytes, remaining queued bytes and
// the candidate size, keeping the configured reserve after all of those finish.
func (s *Service) ensureTaskCapacity(ctx context.Context, task *Task, excludeID uint) error {
	root := s.currentMediaRoot(ctx)
	if task != nil && task.SavePath != "" && !withinStorage(root, task.SavePath) {
		root = task.SavePath
	}
	probe := s.capacityProbe
	if probe == nil {
		probe = disk.Usage
	}
	usage, err := probe(existingStoragePath(root))
	if err != nil {
		return fmt.Errorf("无法读取 %s 的可用空间: %w", root, err)
	}
	var setting model.Setting
	reserveGB := float64(0)
	if err := s.db.WithContext(ctx).Where("key = ?", "media.min_free_gb").Take(&setting).Error; err == nil {
		reserveGB, _ = strconv.ParseFloat(setting.Value, 64)
	}
	if math.IsNaN(reserveGB) || math.IsInf(reserveGB, 0) || reserveGB < 0 || reserveGB > float64(math.MaxInt64)/float64(gib) {
		return fmt.Errorf("最少保留空间设置无效")
	}
	var rows []model.Download
	if err := s.db.WithContext(ctx).Select("id", "save_path", "download_type", "scope", "episode_start", "episode_end", "total_bytes", "downloaded_bytes").
		Where("status IN ? AND id <> ?", []string{model.DownloadStatusPending, model.DownloadStatusDownloading, model.DownloadStatusPaused}, excludeID).Find(&rows).Error; err != nil {
		return err
	}
	queued := int64(0)
	for _, row := range rows {
		if row.SavePath != nil && *row.SavePath != "" && !withinStorage(root, *row.SavePath) {
			continue
		}
		t := Task{DownloadType: row.DownloadType, Scope: row.Scope, EpisodeStart: row.EpisodeStart, EpisodeEnd: row.EpisodeEnd}
		if row.TotalBytes != nil {
			t.EstimatedBytes = *row.TotalBytes
		}
		remaining := estimatedTaskBytes(&t)
		if row.DownloadedBytes != nil && *row.DownloadedBytes > 0 {
			remaining -= *row.DownloadedBytes
		}
		if remaining <= 0 {
			continue
		}
		if remaining > math.MaxInt64-queued {
			queued = math.MaxInt64
			break
		}
		queued += remaining
	}
	required := estimatedTaskBytes(task)
	free := float64(usage.Free)
	reserve := reserveGB * float64(gib)
	if free-float64(queued)-float64(required) < reserve {
		return fmt.Errorf("可用 %.1f GiB，已排队尚需 %.1f GiB，本任务预计 %.1f GiB，需保留 %.1f GiB；空间不足，等待释放空间", free/float64(gib), float64(queued)/float64(gib), float64(required)/float64(gib), reserveGB)
	}
	return nil
}
