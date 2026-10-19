package download

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/service/stream"
)

func (s *Service) streamPreflight(dlID uint, task *Task) func(context.Context, *stream.InterceptedVideo) error {
	return func(ctx context.Context, video *stream.InterceptedVideo) error {
		// Old evidence belongs to the previous execution, never to a refreshed URL.
		s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", dlID).Update("verified_languages", "")
		args := mediacheck.MediaInputArgs("ffprobe", video.URL, video.VideoType, video.Referer, video.UserAgent)
		media, err := mediacheck.ProbeInput(ctx, video.URL, args)
		if err != nil {
			return fmt.Errorf("流媒体预检探测失败: %w", err)
		}
		if task.AnimeID != nil && task.EpisodeNumber != nil {
			var episode model.AnimeEpisode
			if s.db.WithContext(ctx).Where("anime_id = ? AND episode_number = ?", *task.AnimeID, *task.EpisodeNumber).Take(&episode).Error == nil {
				if err := ValidateReferenceDuration(media.Duration(), episode.DurationSeconds); err != nil {
					return fmt.Errorf("流媒体预检不符合要求：%w", err)
				}
			}
		}
		estimate := estimateStreamBytes(media, task.EstimatedBytes)
		if err := s.reserveStreamBytes(ctx, dlID, task, estimate); err != nil {
			return err
		}
		quality, languages, fallback, err := s.mediaGoal(ctx, task.AnimeID)
		if err != nil {
			return err
		}
		if task.Source == SourceManual || fallback {
			return nil
		}
		// A catalog name is not proof of hard-subtitle language.
		assessment := AssessMediaPreferences(media, "", "", quality, languages)
		if assessment.Status == model.QualityUnknown {
			evidence := mediacheck.DetectHardSubtitles(ctx, video.URL, args, media.Duration())
			if evidence.Language != "" {
				withVerifiedLanguages(&media, []string{evidence.Language})
				assessment = AssessMediaPreferences(media, "", task.SourceTitle, quality, languages)
				if assessment.Status == model.QualityMeetsTarget {
					raw, _ := json.Marshal([]string{evidence.Language})
					return s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", dlID).Updates(map[string]interface{}{"verified_languages": string(raw), "quality_note": "预检通过：" + evidence.Reason}).Error
				}
				assessment.Reason = evidence.Reason + "，不符合所选字幕语言"
			} else {
				assessment.Reason = evidence.Reason
			}
		}
		if assessment.Status != model.QualityMeetsTarget {
			return fmt.Errorf("流媒体预检不符合要求：%s", assessment.Reason)
		}
		return nil
	}
}

func withVerifiedLanguages(media *mediacheck.Media, languages []string) {
	media.Streams = append([]mediacheck.Stream(nil), media.Streams...)
	for _, language := range languages {
		tag := ""
		switch language {
		case "simplified":
			tag = "chs"
		case "traditional":
			tag = "cht"
		}
		if tag != "" {
			media.Streams = append(media.Streams, mediacheck.Stream{CodecType: "subtitle", Tags: map[string]string{"language": tag}})
		}
	}
}

func estimateStreamBytes(media mediacheck.Media, fallback int64) int64 {
	if size, _ := strconv.ParseInt(media.Format.Size, 10, 64); size > 0 {
		return size + size/10
	}
	if rate, _ := strconv.ParseFloat(media.Format.BitRate, 64); rate > 0 {
		size := rate * media.Duration() / 8 * 1.15
		if size > 0 && size < float64(math.MaxInt64) {
			return int64(size)
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 2 * gib
}

func (s *Service) reserveStreamBytes(ctx context.Context, dlID uint, task *Task, bytes int64) error {
	s.capacityMu.Lock()
	defer s.capacityMu.Unlock()
	task.EstimatedBytes = bytes
	if err := s.ensureTaskCapacity(ctx, task, dlID); err != nil {
		return fmt.Errorf("媒体存储容量保护: %w", err)
	}
	return s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ?", dlID).Update("total_bytes", bytes).Error
}
