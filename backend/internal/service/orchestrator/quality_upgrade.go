package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/episode"
	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

type libraryQuality struct {
	Needed    []int
	Unknown   int
	Available int
	Finished  bool
}

// auditLibraryQuality records quality debt without clearing playback mappings.
// Fingerprints avoid probing unchanged media on every scheduler pass. Unknown
// evidence is revisited daily, and is never itself a reason to redownload a season.
func (o *Orchestrator) auditLibraryQuality(ctx context.Context, anime *model.Anime, pref Preference) (result libraryQuality, err error) {
	audit := func() error {
		var rows []model.AnimeEpisode
		if err := o.db.WithContext(ctx).Where("anime_id = ?", anime.ID).Order("episode_number").Find(&rows).Error; err != nil {
			return err
		}
		result.Finished = episode.SeasonFinished(anime, rows, time.Now())
		var downloads []model.Download
		if err := o.db.WithContext(ctx).Where("anime_id = ?", anime.ID).Find(&downloads).Error; err != nil {
			return err
		}
		titles := map[string]string{}
		verified := map[string][]string{}
		for _, d := range downloads {
			titles[d.TorrentID] = d.SourceTitle
			if d.VerifiedLanguages != "" {
				var languages []string
				if json.Unmarshal([]byte(d.VerifiedLanguages), &languages) == nil {
					verified[d.TorrentID] = languages
				}
			}
		}
		probe := o.mediaProbe
		if probe == nil {
			probe = mediacheck.Probe
		}
		for _, row := range rows {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !row.Downloaded || row.EpisodeNumber < 1 || row.FilePath == nil || (anime.EpisodeCount != nil && *anime.EpisodeCount > 0 && row.EpisodeNumber > *anime.EpisodeCount) {
				continue
			}
			path := *row.FilePath
			if !libraryPath(path, dlservice.BuildAnimeSavePath(o.currentMediaRoot(ctx), anime)) {
				continue
			}
			info, statErr := os.Stat(path)
			if statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
				continue
			}
			result.Available++
			title := ""
			if row.DownloadID != nil {
				title = titles[*row.DownloadID]
			}
			languages, _ := json.Marshal(pref.Languages)
			evidence := fmt.Sprintf("v1|%s|%d|%d|%s|%s|%s", path, info.Size(), info.ModTime().UnixNano(), title, pref.Quality, languages)
			if row.DownloadID != nil && len(verified[*row.DownloadID]) > 0 {
				raw, _ := json.Marshal(verified[*row.DownloadID])
				evidence += "|verified:" + string(raw)
			}
			sidecars, _ := filepath.Glob(strings.TrimSuffix(path, filepath.Ext(path)) + "*")
			for _, sidecar := range sidecars {
				if st, e := os.Stat(sidecar); e == nil {
					evidence += fmt.Sprintf("|%s:%d:%d", sidecar, st.Size(), st.ModTime().UnixNano())
				}
			}
			fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(evidence)))
			cached := row.QualityFingerprint == fingerprint && row.QualityCheckedAt != nil &&
				(row.QualityStatus != model.QualityUnknown || time.Since(*row.QualityCheckedAt) < 24*time.Hour)
			if !cached {
				assessment := dlservice.QualityAssessment{Status: model.QualityUnknown, Reason: "媒体信息暂不可核实，保留原文件"}
				// An explicitly unrestricted goal needs no codec inspection.
				if pref.Quality == "" && len(pref.Languages) == 0 {
					assessment.Status, assessment.Reason = model.QualityMeetsTarget, ""
				} else if m, e := probe(ctx, path); e == nil {
					var evidence []string
					if row.DownloadID != nil {
						evidence = verified[*row.DownloadID]
					}
					assessment = dlservice.AssessMediaPreferences(m, path, title, pref.Quality, pref.Languages, evidence)
				} else if ctx.Err() != nil {
					return ctx.Err()
				}
				now := time.Now()
				if e := o.db.WithContext(ctx).Model(&model.AnimeEpisode{}).Where("id = ? AND file_path = ? AND downloaded = ?", row.ID, path, true).
					Updates(map[string]interface{}{"quality_status": assessment.Status, "quality_reason": assessment.Reason, "quality_fingerprint": fingerprint, "quality_checked_at": now}).Error; e != nil {
					return e
				}
				row.QualityStatus = assessment.Status
			}
			switch row.QualityStatus {
			case model.QualityUpgradeNeeded:
				result.Needed = append(result.Needed, row.EpisodeNumber)
			case model.QualityUnknown, "":
				result.Unknown++
			}
		}
		return nil
	}
	if o.dlSvc != nil {
		err = o.dlSvc.WithLibraryLock(audit)
	} else {
		err = audit()
	}
	return
}

func libraryPath(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	for _, p := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(p, ".") || strings.EqualFold(p, "Extras") || strings.Contains(strings.ToLower(p), ".part.") || strings.Contains(p, ".anidog-") {
			return false
		}
	}
	return true
}

func (o *Orchestrator) persistQualityState(ctx context.Context, anime *model.Anime, q libraryQuality, expected int) {
	if anime.EpisodeCount != nil && *anime.EpisodeCount > 0 {
		expected = *anime.EpisodeCount
	}
	state := anime.MediaManagementState
	if q.Finished {
		switch {
		case len(q.Needed) > 0:
			state = "upgrading"
		case q.Available < expected:
			state = MediaStateFinalizing
		case q.Unknown > 0:
			state = "quality_review"
		default:
			state = MediaStateArchived
		}
	} else if state == MediaStateArchived || state == "upgrading" || state == "quality_review" {
		state = MediaStateTracking
	}
	if state != anime.MediaManagementState {
		if o.db.WithContext(ctx).Model(&model.Anime{}).Where("id = ?", anime.ID).Update("media_management_state", state).Error == nil {
			anime.MediaManagementState = state
		}
	}
}

func packMeetsUpgradeGoal(title string, pref Preference, expected int) bool {
	p := titleparse.Parse(title)
	if p == nil || !p.IsBatch || p.BatchStart == nil || p.BatchEnd == nil || *p.BatchStart != 1 || *p.BatchEnd != expected {
		return false
	}
	if want := indexer.QualityHeight(pref.Quality); want > 0 && indexer.QualityHeight(p.Quality) < want {
		return false
	}
	if len(pref.Languages) > 0 {
		match := false
		for _, want := range pref.Languages {
			for _, have := range p.Lang {
				if want == have {
					match = true
				}
			}
		}
		if !match {
			return false
		}
	}
	return true
}

// A full, finished season with preference debt is eligible even at 12/12.
// Existing healthy tasks are retained; a compatible in-flight full pack can be
// promoted to an upgrade without downloading a second copy of the same torrent.
func (o *Orchestrator) trySeasonUpgrade(ctx context.Context, anime *model.Anime, q libraryQuality, pref Preference) bool {
	if !q.Finished || len(q.Needed) == 0 || o.dlSvc == nil || !o.dlSvc.HasExecutor(model.DownloadTypeTorrent) {
		return false
	}
	expected := *anime.EpisodeCount
	reason := fmt.Sprintf("完结整季升级：补齐 %d 集，升级 %d 集字幕或画质；旧版保留至全季验证通过", expected-q.Available, len(q.Needed))
	var active []model.Download
	if o.db.WithContext(ctx).Where("anime_id = ? AND status IN ?", anime.ID, []string{model.DownloadStatusPending, model.DownloadStatusDownloading, model.DownloadStatusPaused}).Find(&active).Error != nil {
		return false
	}
	for _, d := range active {
		if d.Purpose == model.DownloadPurposeSeasonUpgrade {
			return true
		}
		if d.Scope == model.DownloadScopeSeason && d.EpisodeStart != nil && d.EpisodeEnd != nil && *d.EpisodeStart == 1 && *d.EpisodeEnd == expected && d.SavePath != nil && dlservice.IsTorrentRaceSavePath(*d.SavePath) && packMeetsUpgradeGoal(d.SourceTitle, pref, expected) && titleparse.InstallmentCompatible(anime.Title, d.SourceTitle) {
			if r := o.db.WithContext(ctx).Model(&model.Download{}).Where("id = ? AND status IN ?", d.ID, []string{model.DownloadStatusPending, model.DownloadStatusDownloading, model.DownloadStatusPaused}).Updates(map[string]interface{}{"purpose": model.DownloadPurposeSeasonUpgrade, "upgrade_reason": reason, "upgrade_target": dlservice.QualityGoalKey(pref.Quality, pref.Languages), "name": fmt.Sprintf("%s - 整季升级 01-%02d", anime.Title, expected)}); r.Error == nil && r.RowsAffected == 1 {
				return true
			}
		}
	}
	if len(active) > 0 {
		return false
	}
	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	}
	if n := titleparse.InstallmentNumber(anime.Title); n > 1 {
		season = n
	}
	strict := pref
	strict.AllowPreferenceFallback = false
	for _, tier := range pref.Priority {
		if (tier != SourceMikan && tier != SourceBT) || pref.IsSourceDisabled(tier) {
			continue
		}
		enabled := o.enabledIndexersForTier(pref, tier)
		if len(enabled) == 0 {
			continue
		}
		candidates := o.collectBTCandidates(ctx, anime, enabled, tier == SourceMikan)
		ranked := indexer.RankByPreferenceAndSeason(filterInstallmentCandidates(anime.Title, candidates), strict.ToIndexerPref(), 0, season)
		// Preserve quality scoring; known live swarms win ties.
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].Score != ranked[j].Score {
				return ranked[i].Score > ranked[j].Score
			}
			return ranked[i].Seeders > ranked[j].Seeders
		})
		for _, c := range ranked {
			if !packMeetsUpgradeGoal(c.Title, pref, expected) || (c.SeedersReported && c.Seeders <= 0) {
				continue
			}
			url := c.MagnetURL
			if url == "" {
				url = c.TorrentURL
			}
			if url == "" {
				continue
			}
			if o.isDuplicateURL(ctx, anime.ID, url) || o.hasAnimeURLFailure(ctx, anime.ID, url) || (c.InfoHash != "" && (o.hasActiveInfoHash(ctx, anime.ID, c.InfoHash) || o.hasHistoricalFailure(ctx, c.InfoHash) || o.hasAnimeInfoHashFailure(ctx, anime.ID, c.InfoHash))) {
				continue
			}
			if err := o.dlSvc.CheckAdditionalCapacity(ctx, c.Size); err != nil {
				o.recordDiag(anime.ID, q.Needed[0], tier, len(candidates), 0, err.Error(), c.Title, c.Score)
				return false
			}
			start, end, trigger := 1, expected, q.Needed[0]
			_, err := o.dlSvc.Create(ctx, &dlservice.Task{Name: fmt.Sprintf("%s - 整季升级 01-%02d", anime.Title, expected), URL: url, SourceTitle: c.Title, EstimatedBytes: c.Size, DownloadType: model.DownloadTypeTorrent, Source: tier, AnimeName: anime.Title, AnimeID: &anime.ID, EpisodeNumber: &trigger, Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end, Purpose: model.DownloadPurposeSeasonUpgrade, UpgradeTarget: dlservice.QualityGoalKey(pref.Quality, pref.Languages), UpgradeReason: reason})
			if err != nil {
				o.recordDiag(anime.ID, trigger, tier, len(candidates), 0, "整季升级入队失败: "+err.Error(), c.Title, c.Score)
				continue
			}
			o.recordDiag(anime.ID, trigger, tier, len(candidates), 0, reason, c.Title, c.Score)
			return true
		}
		o.recordDiag(anime.ID, q.Needed[0], tier, len(candidates), len(candidates), "未命中完整且达到目标的升级合集，保留现有版本", "", 0)
	}
	return false
}
