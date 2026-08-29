package download

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
)

const (
	minStreamPeerSamples       = 2
	maxStreamPeerSamples       = 8
	minStreamPeerDurationRatio = 0.5
	streamDurationTolerance    = 5.0
)

type peerMediaPath struct {
	FilePath string
}

type durationProbeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// validateStreamDurationConsistency prevents a short preview or player fallback
// from winning an episode race merely because it exceeds the absolute minimum.
// Once two completed episodes exist, their median duration becomes the series-
// specific baseline, so short-form anime are judged against themselves rather
// than a hard-coded full-length episode duration.
func (s *Service) validateStreamDurationConsistency(ctx context.Context, task *Task, stagingPath string) error {
	if s == nil || s.db == nil || task == nil || task.AnimeID == nil || task.EpisodeNumber == nil ||
		*task.AnimeID == 0 || *task.EpisodeNumber <= 0 || strings.TrimSpace(stagingPath) == "" {
		return nil
	}

	candidateDuration, err := probeMediaDuration(stagingPath)
	if err != nil {
		return fmt.Errorf("无法复核流媒体成片时长: %w", err)
	}

	var peers []peerMediaPath
	if err := s.db.WithContext(ctx).Model(&model.Download{}).
		Select("file_path").
		Where("anime_id = ? AND episode_number <> ? AND status = ? AND media_missing = ?",
			*task.AnimeID, *task.EpisodeNumber, model.DownloadStatusCompleted, false).
		Where("file_path IS NOT NULL AND file_path <> ''").
		Order("completed_at DESC").
		Limit(maxStreamPeerSamples).
		Scan(&peers).Error; err != nil {
		return fmt.Errorf("查询同番时长基线失败: %w", err)
	}

	peerDurations := make([]float64, 0, len(peers))
	for _, peer := range peers {
		duration, probeErr := probeMediaDuration(peer.FilePath)
		if probeErr == nil {
			peerDurations = append(peerDurations, duration)
		}
	}
	return validateDurationAgainstPeers(candidateDuration, peerDurations)
}

func validateDurationAgainstPeers(candidateDuration float64, peerDurations []float64) error {
	valid := make([]float64, 0, len(peerDurations))
	for _, duration := range peerDurations {
		if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
			valid = append(valid, duration)
		}
	}
	if len(valid) < minStreamPeerSamples {
		return nil
	}
	sort.Float64s(valid)
	median := valid[len(valid)/2]
	if len(valid)%2 == 0 {
		median = (valid[len(valid)/2-1] + median) / 2
	}
	minimum := median * minStreamPeerDurationRatio
	if candidateDuration+streamDurationTolerance < minimum {
		return fmt.Errorf(
			"视频时长明显短于同番剧集: 成片 %.2f 秒，同番中位数 %.2f 秒，低于 %.0f%%",
			candidateDuration, median, minStreamPeerDurationRatio*100,
		)
	}
	return nil
}

func probeMediaDuration(path string) (float64, error) {
	out, err := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "format=duration", "-of", "json", path).Output()
	if err != nil {
		return 0, err
	}
	var parsed durationProbeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, err
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(parsed.Format.Duration), 64)
	if err != nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0, fmt.Errorf("无效时长 %q", parsed.Format.Duration)
	}
	return duration, nil
}
