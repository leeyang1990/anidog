package download

import (
	"bytes"
	"context"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"os"
	"path/filepath"
	"testing"
)

func episodeUpgradeFixture(t *testing.T) (*Service, model.Download, model.Download, *Result) {
	t.Helper()
	s, candidate, result := stagedValidationFixture(t)
	total := 2
	if err := s.db.Model(&model.Anime{}).Where("id = ?", *candidate.AnimeID).Updates(map[string]interface{}{"episode_count": total, "status": model.AnimeStatusFinished}).Error; err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(s.cfg.MediaRoot, "old.mp4")
	if err := os.WriteFile(oldPath, []byte("old playable bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	old := model.Download{TorrentID: "old-owner", Name: "old", URL: "old-source", AnimeID: candidate.AnimeID, EpisodeNumber: candidate.EpisodeNumber, Status: model.DownloadStatusCompleted, FilePath: &oldPath}
	if err := s.db.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	ep := model.AnimeEpisode{AnimeID: *candidate.AnimeID, EpisodeNumber: 1, Downloaded: true, DownloadID: &old.TorrentID, FilePath: &oldPath, QualityStatus: model.QualityUpgradeNeeded}
	if err := s.db.Create(&ep).Error; err != nil {
		t.Fatal(err)
	}
	untouchedPath := filepath.Join(s.cfg.MediaRoot, "unrelated.mp4")
	if err := os.WriteFile(untouchedPath, []byte("unrelated media"), 0600); err != nil {
		t.Fatal(err)
	}
	untouched := model.AnimeEpisode{AnimeID: *candidate.AnimeID, EpisodeNumber: 2, Downloaded: true, FilePath: &untouchedPath, QualityStatus: model.QualityMeetsTarget}
	if err := s.db.Create(&untouched).Error; err != nil {
		t.Fatal(err)
	}
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Language: "simplified", Reason: "independent fixture captions"}
	}
	return s, candidate, old, result
}

func TestVerifiedEpisodeUpgradePreservesOldAssetAndUnrelatedEpisode(t *testing.T) {
	s, candidate, old, result := episodeUpgradeFixture(t)
	incoming, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !s.CompleteEpisodeRace(context.Background(), candidate.ID, result.FilePath, result.FinalPath) {
		t.Fatal("verified upgrade lost to playable but suboptimal media")
	}
	var saved, previous model.Download
	s.db.First(&saved, candidate.ID)
	s.db.First(&previous, old.ID)
	if saved.Status != model.DownloadStatusCompleted || saved.ValidationPending || previous.Status != model.DownloadStatusSuperseded || previous.FilePath == nil {
		t.Fatal(saved, previous)
	}
	original, err := os.ReadFile(*previous.FilePath)
	if err != nil || string(original) != "old playable bytes" {
		t.Fatal("original not preserved", err)
	}
	promoted, err := os.ReadFile(result.FinalPath)
	if err != nil || !bytes.Equal(promoted, incoming) {
		t.Fatal("new media changed", err)
	}
	var eps []model.AnimeEpisode
	s.db.Where("anime_id = ?", *candidate.AnimeID).Order("episode_number").Find(&eps)
	if len(eps) != 2 || eps[0].DownloadID == nil || *eps[0].DownloadID != candidate.TorrentID || eps[0].QualityStatus != model.QualityMeetsTarget || eps[1].FilePath == nil || *eps[1].FilePath == *old.FilePath {
		t.Fatal("replacement changed unrelated episode", eps)
	}
	unrelated, err := os.ReadFile(*eps[1].FilePath)
	if err != nil || string(unrelated) != "unrelated media" {
		t.Fatal("unrelated asset changed", err)
	}
	if err = s.RecoverMediaUpgrades(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.CompleteEpisodeRace(context.Background(), candidate.ID, result.FilePath, result.FinalPath) {
		t.Fatal("late callback repeated replacement")
	}
}

func TestEpisodeUpgradeRequiresConfirmedDebtAndStrictTarget(t *testing.T) {
	for _, scenario := range []string{"unknown debt", "not finished", "manual fallback traditional", "subtitle unknown"} {
		t.Run(scenario, func(t *testing.T) {
			s, candidate, old, result := episodeUpgradeFixture(t)
			switch scenario {
			case "unknown debt":
				s.db.Model(&model.AnimeEpisode{}).Where("anime_id = ? AND episode_number = 1", *candidate.AnimeID).Update("quality_status", model.QualityUnknown)
			case "not finished":
				s.db.Model(&model.Anime{}).Where("id = ?", *candidate.AnimeID).Update("status", model.AnimeStatusOngoing)
			case "manual fallback traditional":
				s.db.Model(&candidate).Update("source", SourceManual)
				s.db.Create(&model.Setting{Key: "download.allow_preference_fallback", Value: "true"})
				s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
					return mediacheck.SubtitleEvidence{Language: "traditional"}
				}
			case "subtitle unknown":
				s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
					return mediacheck.SubtitleEvidence{}
				}
			}
			if s.CompleteEpisodeRace(context.Background(), candidate.ID, result.FilePath, result.FinalPath) {
				t.Fatal("nonqualifying replacement accepted")
			}
			data, err := os.ReadFile(*old.FilePath)
			if err != nil || string(data) != "old playable bytes" {
				t.Fatal("playable original changed", err)
			}
			var previous model.Download
			s.db.First(&previous, old.ID)
			if previous.Status != model.DownloadStatusCompleted {
				t.Fatal("original ownership changed", previous)
			}
		})
	}
}

func TestEpisodeUpgradeSharesAmbiguousCommitRecovery(t *testing.T) {
	for _, mode := range []string{"committed", "not committed", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			s, candidate, old, result := episodeUpgradeFixture(t)
			conn, err := s.db.DB()
			if err != nil {
				t.Fatal(err)
			}
			pool := &commitAckPool{ConnPool: conn, mode: mode}
			previousPool := s.db.Statement.ConnPool
			s.db.Statement.ConnPool = pool
			defer func() { s.db.Statement.ConnPool = previousPool }()
			s.RegisterExecutor(candidate.DownloadType, &upgradeFileGate{pause: func() error { pool.inject.Store(true); return nil }})
			won := s.CompleteEpisodeRace(context.Background(), candidate.ID, result.FilePath, result.FinalPath)
			if pool.inject.Load() {
				t.Fatal("swap commit not reached")
			}
			if mode == "unknown" {
				if won {
					t.Fatal("unknown commit reported successful")
				}
				pool.failRead.Store(false)
			}
			if err = s.RecoverMediaUpgrades(context.Background()); err != nil {
				t.Fatal(err)
			}
			var task model.Download
			s.db.First(&task, candidate.ID)
			if mode == "not committed" {
				if won || task.Status != model.DownloadStatusPaused {
					t.Fatal("uncommitted upgrade not paused", won, task.Status)
				}
				data, err := os.ReadFile(*old.FilePath)
				if err != nil || string(data) != "old playable bytes" {
					t.Fatal("rollback lost original", err)
				}
				if _, err = os.Stat(result.FilePath); err != nil {
					t.Fatal("rollback lost incoming", err)
				}
			} else if task.Status != model.DownloadStatusCompleted || task.BackupPath == "" {
				t.Fatal("committed upgrade lost", task.Status)
			}
		})
	}
}

func TestUpgradeDestinationSymlinkCannotMoveOriginal(t *testing.T) {
	s, candidate, old, result := episodeUpgradeFixture(t)
	outside := t.TempDir()
	link := filepath.Join(s.cfg.MediaRoot, "linked-destination")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(link, "episode.mp4")
	if s.CompleteEpisodeRace(context.Background(), candidate.ID, result.FilePath, final) {
		t.Fatal("unsafe destination accepted")
	}
	data, err := os.ReadFile(*old.FilePath)
	if err != nil || string(data) != "old playable bytes" {
		t.Fatal("original moved before path validation", err)
	}
	if _, err = os.Stat(filepath.Join(outside, "episode.mp4")); !os.IsNotExist(err) {
		t.Fatal("wrote outside media root", err)
	}
	var row model.Download
	s.db.First(&row, candidate.ID)
	if row.Status != model.DownloadStatusPaused {
		t.Fatal("unsafe swap repeated indefinitely", row.Status)
	}
}
