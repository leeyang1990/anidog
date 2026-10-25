package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/mediaquality"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/service/mediapolicy"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

type candidateMediaError struct{ error }

func (e candidateMediaError) Unwrap() error { return e.error }

func (s *Service) validateCompletedMedia(ctx context.Context, dl *model.Download, ep int, path string) error {
	return s.validateMedia(ctx, dl, ep, path, false)
}

func (s *Service) validateMedia(ctx context.Context, dl *model.Download, ep int, path string, force bool) error {
	if !force && (s.cfg == nil || (!s.cfg.VerifyDownloadedMedia && dl.Source != SourceStream)) && dl.Purpose != model.DownloadPurposeSeasonUpgrade {
		return nil
	}
	m, err := mediacheck.Probe(ctx, path)
	if err != nil {
		if mediacheck.IsToolError(err) {
			return failure.Wrap(failure.Validation, failure.InspectionFailed, err)
		}
		return candidateMediaError{err}
	}
	if dl.AnimeID != nil {
		var reference model.AnimeEpisode
		err := s.db.WithContext(ctx).Where("anime_id = ? AND episode_number = ?", *dl.AnimeID, ep).First(&reference).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return failure.Wrap(failure.Validation, failure.StateUnavailable, err)
		}
		if err == nil {
			if err := ValidateReferenceDuration(m.Duration(), reference.DurationSeconds); err != nil {
				return candidateMediaError{err}
			}
		}
	}
	if err := mediacheck.CheckTail(ctx, path, m.Duration()); err != nil {
		if mediacheck.IsToolError(err) {
			return failure.Wrap(failure.Validation, failure.InspectionFailed, err)
		}
		return candidateMediaError{err}
	}
	// Manual selection can explicitly accept a version different from global
	// preferences, but cannot waive structural validity or reference duration.
	if dl.Source == SourceManual && dl.Purpose != model.DownloadPurposeSeasonUpgrade {
		return nil
	}
	quality, languages, fallback, err := s.mediaGoal(ctx, dl.AnimeID)
	if err != nil {
		return err
	}
	if fallback && dl.Purpose != model.DownloadPurposeSeasonUpgrade {
		return nil
	}
	// Source titles rank candidates. Admission requires the actual media, for
	// BT and streams alike. Old unbound verified_languages cannot grant entry.
	assessment := AssessMediaPreferences(m, path, "", quality, languages)
	tracks := subtitleTrackLanguages(m)
	if len(tracks) > 0 && assessment.Status == model.QualityMeetsTarget {
		evidence := mediacheck.SubtitleEvidence{InspectorVersion: mediacheck.SubtitleInspectorVersion, Method: "subtitle_tracks", TrackLanguages: tracks, Reason: "实际字幕轨符合要求", Duration: m.Duration()}
		evidence.Width, evidence.Height = m.Resolution()
		for _, lang := range tracks {
			if lang == "simplified" || lang == "traditional" {
				evidence.Language = lang
				if lang == "simplified" {
					break
				}
			}
		}
		if err = s.saveSubtitleEvidence(ctx, dl, "", evidence, nil); err != nil {
			return err
		}
	}
	w, h := m.Resolution()
	want := mediaquality.Height(quality)
	resolutionOK := want <= 0 || h >= want || w >= want*16/9 && h >= want*7/10
	if len(languages) > 0 && resolutionOK && assessment.Status != model.QualityMeetsTarget {
		evidence, err := s.inspectContent(ctx, dl, path, m)
		if err != nil {
			return err
		}
		if evidence.Retryable {
			return failure.Wrap(failure.Subtitles, failure.InspectionFailed, fmt.Errorf("成片字幕检查暂时不可用：%s", evidence.Reason))
		}
		if evidence.Language != "" {
			withVerifiedLanguages(&m, []string{evidence.Language})
			assessment = AssessMediaPreferences(m, path, "", quality, languages)
		}
		if evidence.Language == "" && assessment.Status == model.QualityUnknown {
			return failure.Wrap(failure.Subtitles, failure.SubtitleUnknown, fmt.Errorf("字幕待复核：%s；%s；已保留成片，尚未入库", assessment.Reason, evidence.Reason))
		}
	}
	if assessment.Status != model.QualityMeetsTarget {
		return candidateMediaError{failure.Wrap(failure.Validation, failure.PreferenceMismatch, fmt.Errorf("%s", assessment.Reason))}
	}
	return nil
}

func subtitleTrackLanguages(media mediacheck.Media) []string {
	seen := map[string]bool{}
	for _, st := range media.Streams {
		if st.CodecType != "subtitle" {
			continue
		}
		languages := mediacheck.SubtitleLanguages(st.Tags)
		for _, language := range languages {
			seen[language] = true
		}
		if len(languages) == 0 && st.Tags["language"] != "" {
			seen[st.Tags["language"]] = true
		}
	}
	var result []string
	for lang := range seen {
		result = append(result, lang)
	}
	sort.Strings(result)
	return result
}

func QualityGoalKey(quality string, languages []string) string {
	normalized := append([]string(nil), languages...)
	sort.Strings(normalized)
	raw, _ := json.Marshal(struct {
		Quality   string
		Languages []string
	}{quality, normalized})
	return string(raw)
}

func (s *Service) mediaGoal(ctx context.Context, animeID *uint) (string, []string, bool, error) {
	goal, err := mediapolicy.Load(ctx, s.db, animeID)
	if err != nil {
		return "", nil, false, failure.Wrap(failure.Validation, failure.StateUnavailable, err)
	}
	return goal.Quality, goal.Languages, goal.AllowFallback, nil
}

type QualityAssessment struct {
	Status string
	Reason string
}

func validateMediaPreferences(m mediacheck.Media, path, title, quality string, languages []string) error {
	assessment := AssessMediaPreferences(m, path, title, quality, languages)
	if assessment.Status != model.QualityMeetsTarget {
		return fmt.Errorf("%s", assessment.Reason)
	}
	return nil
}

// AssessMediaPreferences keeps preference debt separate from playback availability.
// Unknown hard subtitles do not prove that the existing file needs replacement.
func AssessMediaPreferences(m mediacheck.Media, path, title, quality string, languages []string, verified ...[]string) QualityAssessment {
	if len(verified) > 0 {
		withVerifiedLanguages(&m, verified[0])
	}
	w, h := m.Resolution()
	want := mediaquality.Height(quality)
	// Cinema crops can retain the full width of a 1080p/2160p master.
	if want > 0 && h < want && !(w >= want*16/9 && h >= want*7/10) {
		return QualityAssessment{model.QualityUpgradeNeeded, fmt.Sprintf("实际分辨率 %dx%d 低于要求 %s", w, h, quality)}
	}
	if len(languages) == 0 {
		return QualityAssessment{Status: model.QualityMeetsTarget}
	}
	have := map[string]bool{}
	parsedLanguages := titleparse.Parse(title).Lang
	knownSubtitle := len(parsedLanguages) > 0
	addSubtitle := func(tags map[string]string) {
		for _, language := range mediacheck.SubtitleLanguages(tags) {
			have[language] = true
		}
	}
	for _, st := range m.Streams {
		if st.CodecType == "subtitle" {
			addSubtitle(st.Tags)
		}
		if st.CodecType == "audio" && st.Tags["language"] == "jpn" {
			have["japanese"] = true
		}
		if st.CodecType == "audio" && st.Tags["language"] == "eng" {
			have["english"] = true
		}
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	var siblings []string
	if path != "" {
		siblings, _ = filepath.Glob(base + "*")
	}
	for _, p := range siblings {
		switch strings.ToLower(filepath.Ext(p)) {
		case ".ass", ".ssa", ".srt", ".vtt":
			addSubtitle(map[string]string{"title": filepath.Base(p)})
		}
	}
	// Actual Chinese tracks/sidecars take precedence over a pack title. Only
	// use title evidence for hard subtitles when the file has no such evidence.
	actualChinese := have["simplified"] || have["traditional"]
	for _, lang := range parsedLanguages {
		if !actualChinese || (lang != "simplified" && lang != "traditional") {
			have[lang] = true
		}
	}
	for _, lang := range languages {
		if have[lang] {
			return QualityAssessment{Status: model.QualityMeetsTarget}
		}
	}
	status := model.QualityUnknown
	if knownSubtitle || have["simplified"] || have["traditional"] {
		status = model.QualityUpgradeNeeded
	}
	var detected []string
	for language := range have {
		detected = append(detected, language)
	}
	sort.Strings(detected)
	if status == model.QualityUpgradeNeeded {
		return QualityAssessment{status, fmt.Sprintf("已识别字幕/语言 %s，不符合要求 %s", strings.Join(detected, ","), strings.Join(languages, ","))}
	}
	return QualityAssessment{status, fmt.Sprintf("尚无满足字幕/语言要求 %s 的证据（已检测到：%s）", strings.Join(languages, ","), strings.Join(detected, ","))}
}

// Reject before moving into the library. Retain the candidate's bytes so a
// metadata correction or explicit user choice remains recoverable.
func (s *Service) rejectMediaCandidate(ctx context.Context, dl *model.Download, err error) {
	message := "成片校验未通过：" + err.Error()
	var rejected candidateMediaError
	if !errors.As(err, &rejected) {
		updates, _, _ := failureFields(err, dl.RetryCount)
		updates["last_error"], updates["quality_note"] = message, message
		if dl.FilePath != nil {
			updates["file_path"] = *dl.FilePath
		}
		s.updateStatus(dl.ID, model.DownloadStatusFailed, updates)
		if failure.Describe(err).Kind == model.FailureKindReview {
			if executor := s.executors[dl.DownloadType]; executor != nil {
				_ = executor.Remove(executorTaskID(dl), false)
			}
			if dl.AnimeID != nil && dl.EpisodeNumber != nil {
				s.triggerRejectedStreamRecovery(validationTask(dl))
			}
		}
		return
	}
	updates, _, _ := failureFields(failure.Wrap(failure.Validation, failure.InvalidMedia, err), dl.RetryCount)
	updates["last_error"], updates["quality_note"], updates["seeking_alternative"] = message, message, false
	updates["validation_pending"], updates["validation_final_path"] = false, ""
	if dl.FilePath != nil {
		updates["file_path"] = *dl.FilePath
	}
	if !s.updateStatus(dl.ID, model.DownloadStatusFailed, updates) {
		return
	}
	if executor := s.executors[dl.DownloadType]; executor != nil {
		_ = executor.Remove(executorTaskID(dl), false)
	}
	if dl.AnimeID != nil && dl.EpisodeNumber != nil && s.rejectedStreamHandler != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			s.rejectedStreamHandler(ctx, *dl.AnimeID, *dl.EpisodeNumber)
		}()
	}
}

func existingCompletionPath(staging, final string) string {
	if staging != "" {
		if _, err := os.Stat(staging); err == nil {
			return staging
		}
	}
	return final
}
