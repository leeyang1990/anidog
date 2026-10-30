package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/service/stream"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type refreshUnknownEvidence struct{}

// Provenance belongs to the inspection result, not another attempt that happens
// to contain the same video. A new tool failure must never inherit old proof.
type contentInspection struct {
	evidence mediacheck.SubtitleEvidence
	origin   *uint
}

func (s *Service) inspectContent(ctx context.Context, dl *model.Download, path string, media mediacheck.Media) (mediacheck.SubtitleEvidence, error) {
	var empty mediacheck.SubtitleEvidence
	before, err := os.Stat(path)
	if err != nil {
		return empty, failure.Wrap(failure.Subtitles, failure.StorageUnavailable, err)
	}
	codec := ""
	for _, st := range media.Streams {
		if st.CodecType == "video" {
			codec = st.CodecName
			break
		}
	}
	hash := s.videoHasher
	if hash == nil {
		hash = mediacheck.VideoDigest
	}
	digest, err := hash(ctx, path, codec)
	if err != nil {
		return empty, failure.Wrap(failure.Subtitles, failure.InspectionFailed, err)
	}
	key := digest + ":" + mediacheck.SubtitleInspectorVersion
	result := s.evidenceFlights.DoChan(key, func() (interface{}, error) {
		refresh, _ := ctx.Value(refreshUnknownEvidence{}).(bool)
		var cached model.MediaEvidence
		err := s.db.WithContext(ctx).Where("video_hash = ? AND inspector_version = ?", digest, mediacheck.SubtitleInspectorVersion).Take(&cached).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return empty, failure.Wrap(failure.Subtitles, failure.StateUnavailable, err)
		}
		if err == nil {
			var evidence mediacheck.SubtitleEvidence
			if err = json.Unmarshal([]byte(cached.Evidence), &evidence); err != nil {
				return empty, failure.Wrap(failure.Subtitles, failure.StateUnavailable, err)
			}
			if evidence.InspectorVersion == mediacheck.SubtitleInspectorVersion && !evidence.Retryable && !(refresh && evidence.Language == "") {
				return contentInspection{evidence, cached.OriginDownloadID}, nil
			}
		}
		var evidence mediacheck.SubtitleEvidence
		var reassessed bool
		var previous mediacheck.SubtitleEvidence
		w, h := media.Resolution()
		if !refresh && dl.VideoHash == digest && json.Unmarshal([]byte(dl.SubtitleEvidence), &previous) == nil && previous.Width == w && previous.Height == h && math.Abs(previous.Duration-media.Duration()) < 0.01 {
			evidence, reassessed = mediacheck.ReassessRetainedSubtitles(previous)
		}
		if !reassessed {
			inspect := s.subtitleInspector
			if inspect == nil {
				inspect = mediacheck.DetectHardSubtitles
			}
			evidence = inspect(ctx, path, nil, media.Duration())
		}
		evidence.InspectorVersion = mediacheck.SubtitleInspectorVersion
		evidence.Method = "hard_subtitles"
		evidence.Width, evidence.Height = media.Resolution()
		evidence.Duration = media.Duration()
		after, err := os.Stat(path)
		if err != nil {
			return empty, err
		}
		if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return empty, fmt.Errorf("字幕检查期间成片发生变化，请重新检查")
		}
		// Tool failures are attempt facts, not immutable content facts.
		var origin *uint
		if reassessed && dl.EvidenceReusedFrom != nil {
			origin = dl.EvidenceReusedFrom
		} else if dl.ID != 0 {
			id := dl.ID
			origin = &id
		}
		if !evidence.Retryable {
			raw, err := json.Marshal(evidence)
			if err != nil {
				return empty, err
			}
			record := model.MediaEvidence{VideoHash: digest, InspectorVersion: mediacheck.SubtitleInspectorVersion, Evidence: string(raw), OriginDownloadID: origin}
			if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "video_hash"}, {Name: "inspector_version"}}, DoUpdates: clause.AssignmentColumns([]string{"evidence", "origin_download_id", "updated_at"})}).Create(&record).Error; err != nil {
				return empty, failure.Wrap(failure.Subtitles, failure.StateUnavailable, err)
			}
		}
		return contentInspection{evidence, origin}, nil
	})
	select {
	case <-ctx.Done():
		return empty, ctx.Err()
	case completed := <-result:
		if completed.Err != nil {
			return empty, failure.Wrap(failure.Subtitles, failure.InspectionFailed, completed.Err)
		}
		after, statErr := os.Stat(path)
		if statErr != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return empty, failure.Wrap(failure.Subtitles, failure.InspectionFailed, fmt.Errorf("检查期间成片变化，不能复用旧证据"))
		}
		inspection := completed.Val.(contentInspection)
		evidence := inspection.evidence
		evidence.TrackLanguages = subtitleTrackLanguages(media)
		var reused *uint
		if inspection.origin != nil && *inspection.origin != dl.ID {
			reused = inspection.origin
		}
		if err = s.saveSubtitleEvidence(ctx, dl, digest, evidence, reused); err != nil {
			return empty, err
		}
		return evidence, nil
	}
}

func (s *Service) saveSubtitleEvidence(ctx context.Context, dl *model.Download, digest string, evidence mediacheck.SubtitleEvidence, reused *uint) error {
	raw, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	languages := ""
	if evidence.Language != "" {
		value, _ := json.Marshal([]string{evidence.Language})
		languages = string(value)
	}
	if dl.ID != 0 {
		q := s.db.WithContext(ctx).Model(&model.Download{}).Where("id = ? AND status = ?", dl.ID, dl.Status).Updates(map[string]interface{}{"video_hash": digest, "subtitle_evidence": string(raw), "verified_languages": languages, "evidence_reused_from": reused})
		if q.Error != nil {
			return failure.Wrap(failure.Subtitles, failure.StateUnavailable, q.Error)
		}
		if q.RowsAffected != 1 {
			return failure.Wrap(failure.Subtitles, failure.Canceled, fmt.Errorf("字幕检查所属任务已结束"))
		}
	}
	dl.VideoHash, dl.SubtitleEvidence, dl.VerifiedLanguages, dl.EvidenceReusedFrom = digest, string(raw), languages, reused
	return nil
}

// Known rejected captions can stop a repeat transfer, even when another source
// remuxes the video. Native subtitle tracks are checked first, so an alternate
// track cannot inherit a rejection from an otherwise identical video.
func (s *Service) rejectKnownContent(ctx context.Context, dlID uint, task *Task, video *stream.InterceptedVideo, media mediacheck.Media, quality string, languages []string) error {
	if task.AnimeID == nil || task.EpisodeNumber == nil {
		return nil
	}
	var candidates []model.Download
	if err := s.db.WithContext(ctx).Where("anime_id = ? AND episode_number = ? AND id <> ? AND video_hash <> '' AND subtitle_evidence <> ''", *task.AnimeID, *task.EpisodeNumber, dlID).Order("id DESC").Limit(8).Find(&candidates).Error; err != nil {
		return failure.Wrap(failure.Probe, failure.StateUnavailable, err)
	}
	seen := map[string]bool{}
	proofCtx, stopProof := context.WithTimeout(ctx, 45*time.Second)
	defer stopProof()
	for _, old := range candidates {
		if proofCtx.Err() != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		}
		if seen[old.VideoHash] {
			continue
		}
		seen[old.VideoHash] = true
		var evidence mediacheck.SubtitleEvidence
		if json.Unmarshal([]byte(old.SubtitleEvidence), &evidence) != nil || evidence.InspectorVersion != mediacheck.SubtitleInspectorVersion || evidence.Retryable {
			continue
		}
		w, h := media.Resolution()
		if w != evidence.Width || h != evidence.Height || math.Abs(media.Duration()-evidence.Duration) > 1 {
			continue
		}
		assessed := media
		withVerifiedLanguages(&assessed, []string{evidence.Language})
		if AssessMediaPreferences(assessed, "", "", quality, languages).Status == model.QualityMeetsTarget {
			continue
		}
		matched, err := mediacheck.MatchSubtitleProof(proofCtx, video.URL, video.InputArgs("ffmpeg"), evidence)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || !matched {
			continue
		}
		row := model.Download{ID: dlID, Status: model.DownloadStatusDownloading}
		// A frame match refers to old proof; it does not assert a full digest for
		// bytes that have never been downloaded in this attempt.
		if err = s.saveSubtitleEvidence(ctx, &row, "", evidence, &old.ID); err != nil {
			return err
		}
		return failure.Wrap(failure.Subtitles, failure.DuplicateMedia, fmt.Errorf("已复用任务 %d 的字幕证据，跳过重复传输：%s", old.ID, AssessMediaPreferences(assessed, "", "", quality, languages).Reason))
	}
	return nil
}
