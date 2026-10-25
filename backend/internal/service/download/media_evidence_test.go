package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
)

func TestContentEvidenceSurvivesRejectionAndPolicyChange(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	var calls atomic.Int32
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		calls.Add(1)
		return mediacheck.SubtitleEvidence{Language: "traditional", Reason: "confirmed traditional", Attempted: 3, Extracted: 3, Recognized: 3, Frames: 3}
	}
	ctx := context.Background()
	err := s.validateCompletedMedia(ctx, &row, 1, result.FilePath)
	if failure.Describe(err).Code != failure.PreferenceMismatch {
		t.Fatal(err)
	}
	var saved model.Download
	s.db.First(&saved, row.ID)
	if saved.VideoHash == "" || saved.VerifiedLanguages != `["traditional"]` || saved.SubtitleEvidence == "" {
		t.Fatal("rejected facts were discarded", saved)
	}
	// A restart shares durable facts, rather than only a process-local cache.
	other := NewService(s.db, s.cfg, nil)
	other.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		t.Fatal("same video was OCR'd after restart")
		return mediacheck.SubtitleEvidence{}
	}
	copyRow := row
	copyRow.ID = 0
	copyRow.TorrentID = "other-source"
	copyRow.VideoHash = ""
	copyRow.VerifiedLanguages = ""
	copyRow.SubtitleEvidence = ""
	s.db.Create(&copyRow)
	if err = other.validateCompletedMedia(ctx, &copyRow, 1, result.FilePath); failure.Describe(err).Code != failure.PreferenceMismatch {
		t.Fatal(err)
	}
	if copyRow.EvidenceReusedFrom == nil || *copyRow.EvidenceReusedFrom != row.ID {
		t.Fatal("missing source-independent provenance")
	}
	if err = s.db.Model(&model.Setting{}).Where("key = ?", "download.languages").Update("value", `["traditional"]`).Error; err != nil {
		t.Fatal(err)
	}
	// Use an explicit per-anime preference to test the policy boundary regardless
	// of legacy global setting names.
	if err = s.db.Model(&model.Anime{}).Where("id = ?", *row.AnimeID).Update("override_languages", `["traditional"]`).Error; err != nil {
		t.Fatal(err)
	}
	if err = other.validateCompletedMedia(ctx, &copyRow, 1, result.FilePath); err != nil {
		t.Fatal("facts were cached as a policy verdict", err)
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
}

func TestInconclusiveEvidenceRetainsPayloadAndDoesNotAutoRetry(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Attempted: 10, Extracted: 10, Recognized: 10, Reason: "no valid glyphs"}
	}
	if s.CompleteEpisodeRace(context.Background(), row.ID, result.FilePath, result.FinalPath) {
		t.Fatal("unknown subtitles archived")
	}
	s.db.First(&row, row.ID)
	if row.FailureKind != model.FailureKindReview || !row.ValidationPending || failure.ResultClass(row) != "review" {
		t.Fatal(row)
	}
	if err := s.ResumeMediaValidation(context.Background(), *row.AnimeID); err != nil {
		t.Fatal(err)
	}
	s.db.First(&row, row.ID)
	if row.RetryCount != 0 || row.Status != model.DownloadStatusFailed {
		t.Fatal("review resurrected itself")
	}
	if _, err := os.Stat(result.FilePath); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceCacheSingleFlightAndChangedContent(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	media, err := mediacheck.Probe(context.Background(), result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	s.videoHasher = func(context.Context, string, string) (string, error) { return "video-test-a", nil }
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return mediacheck.SubtitleEvidence{Language: "traditional"}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		copyRow := row
		copyRow.ID = 0
		copyRow.TorrentID = fmt.Sprint("parallel-", i)
		s.db.Create(&copyRow)
		wg.Add(1)
		go func(row model.Download) {
			defer wg.Done()
			_, err := s.inspectContent(context.Background(), &row, result.FilePath, media)
			errs <- err
		}(copyRow)
	}
	<-entered
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("concurrent OCR duplicated", calls.Load())
	}
	s.videoHasher = func(context.Context, string, string) (string, error) { return "video-test-b", nil }
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		calls.Add(1)
		return mediacheck.SubtitleEvidence{Language: "simplified"}
	}
	evidence, err := s.inspectContent(context.Background(), &row, result.FilePath, media)
	if err != nil || evidence.Language != "simplified" || calls.Load() != 2 {
		t.Fatal("new bytes inherited old proof", evidence, err)
	}
}

func TestToolFailureIsPersistedButNotCachedAsContent(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Retryable: true, Reason: "ocr timeout", Attempted: 2, FrameErrors: 1}
	}
	err := s.validateCompletedMedia(context.Background(), &row, 1, result.FilePath)
	if failure.Describe(err).Code != failure.InspectionFailed {
		t.Fatal(err)
	}
	var evidence mediacheck.SubtitleEvidence
	if json.Unmarshal([]byte(row.SubtitleEvidence), &evidence) != nil || !evidence.Retryable {
		t.Fatal("attempt facts missing")
	}
	var count int64
	s.db.Model(&model.MediaEvidence{}).Count(&count)
	if count != 0 {
		t.Fatal("outage poisoned content cache")
	}
}

func TestCurrentEpisodeProjectionAndUntruncatedCounts(t *testing.T) {
	s := capacityService(t, 500*gib)
	ctx := context.Background()
	animeID := uint(1)
	ep := 1
	for i := 0; i < 510; i++ {
		if err := s.db.Create(&model.Download{TorrentID: fmt.Sprint("old-", i), Name: "A", URL: "x", AnimeID: &animeID, EpisodeNumber: &ep, Status: model.DownloadStatusFailed, FailureCode: string(failure.SubtitleUnknown), FailureKind: model.FailureKindReview}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{model.DownloadStatusCompleted, model.DownloadStatusFailed} {
		if err := s.db.Create(&model.Download{TorrentID: status, Name: "A", URL: "x", AnimeID: &animeID, EpisodeNumber: &ep, Status: status}).Error; err != nil {
			t.Fatal(err)
		}
	}
	ep2 := 2
	s.db.Create(&model.Download{TorrentID: "b", Name: "B", URL: "x", AnimeID: &animeID, EpisodeNumber: &ep2, Status: model.DownloadStatusFailed, FailureCode: string(failure.PreferenceMismatch)})
	r, err := s.CurrentTasks(ctx, "all", "", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 2 || r.AttemptTotal != 513 || r.Counts["completed"] != 1 || r.Counts["rejected"] != 1 {
		t.Fatal(r)
	}
	for _, row := range r.Items {
		if *row.EpisodeNumber == 1 && row.AttemptCount != 512 {
			t.Fatal("attempt count truncated", row.AttemptCount)
		}
	}
	r, err = s.CurrentTasks(ctx, "rejected", "B", 1, 1)
	if err != nil || len(r.Items) != 1 || r.Items[0].Name != "B" {
		t.Fatal(r, err)
	}
	history, err := s.AttemptHistory(ctx, r.Items[0].ID, 1, 100)
	if err != nil || history.Total != 1 {
		t.Fatal(history, err)
	}
	for _, row := range rowsForCurrentTest(t, s) {
		if row.Status == model.DownloadStatusCompleted {
			history, err = s.AttemptHistory(ctx, row.ID, 6, 100)
			if err != nil || history.Total != 512 || len(history.Items) != 12 {
				t.Fatal("history truncated", history, err)
			}
			break
		}
	}
}

func rowsForCurrentTest(t *testing.T, s *Service) []model.Download {
	t.Helper()
	var rows []model.Download
	if err := s.db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestBackgroundReinspectionUsesRuntimeRootAndNeverArchives(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	if err := s.db.Create(&model.Setting{Key: "download_dir", Value: s.cfg.MediaRoot}).Error; err != nil {
		t.Fatal(err)
	}
	// Production uses the persisted library setting with an empty/different
	// config fallback. Both the request and worker must resolve the same root.
	s.cfg.MediaRoot = t.TempDir()
	s.db.Model(&row).Update("status", model.DownloadStatusFailed)
	s.cfg.VerifyDownloadedMedia = false
	entered := make(chan struct{})
	release := make(chan struct{})
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		close(entered)
		<-release
		return mediacheck.SubtitleEvidence{Language: "traditional"}
	}
	started, err := s.StartRetainedInspection(context.Background(), row.ID)
	if err != nil || !started.InspectionActive {
		t.Fatal(started, err)
	}
	<-entered
	if _, err = s.StartRetainedInspection(context.Background(), row.ID); err == nil {
		t.Fatal("duplicate background inspection admitted")
	}
	state, err := s.GetByID(context.Background(), row.ID)
	if err != nil || !state.InspectionActive {
		t.Fatal(state, err)
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, err = s.GetByID(context.Background(), row.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !state.InspectionActive {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if state.InspectionActive || state.Status != model.DownloadStatusFailed || state.VerifiedLanguages != `["traditional"]` || state.FailureCode != string(failure.PreferenceMismatch) {
		t.Fatal(state)
	}
	if _, err = os.Stat(result.FinalPath); !os.IsNotExist(err) {
		t.Fatal("inspection promoted library file")
	}
}

func TestRetainedBTUsesRuntimeRootAcrossStagingAndLibrary(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	root := s.cfg.MediaRoot
	if err := s.db.Create(&model.Setting{Key: "download_dir", Value: root}).Error; err != nil {
		t.Fatal(err)
	}
	stagingDir := filepath.Join(root, ".anidog-race", "anime-1", "episode-001", "candidate")
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(stagingDir, "episode.mp4")
	if err := os.Rename(result.FilePath, staging); err != nil {
		t.Fatal(err)
	}
	row.DownloadType = model.DownloadTypeTorrent
	row.FilePath, row.SavePath = &staging, &stagingDir
	s.cfg.MediaRoot = t.TempDir()
	retained, err := s.retainedValidationResult(context.Background(), &row)
	if err != nil || retained == nil || retained.FilePath != staging || retained.FinalPath != result.FinalPath {
		t.Fatal("BT recovery ignored effective root or treated staging as library root", retained, err)
	}
	outside := filepath.Join(s.cfg.MediaRoot, "outside.mp4")
	if err := os.WriteFile(outside, []byte("not a library file"), 0600); err != nil {
		t.Fatal(err)
	}
	row.FilePath = &outside
	if _, err = s.retainedValidationResult(context.Background(), &row); err == nil {
		t.Fatal("config fallback expanded access beyond the effective root")
	}
}

func TestArchivedPackResolutionRequiresOwnedAvailableFilesAndPreservesHistory(t *testing.T) {
	for _, scenario := range []string{"available", "missing file", "missing owner", "unmapped", "upgrade"} {
		t.Run(scenario, func(t *testing.T) {
			s := capacityService(t, 500*gib)
			anime := model.Anime{Title: "pack archive"}
			s.db.Create(&anime)
			start, end, ep := 1, 12, 2
			pack := model.Download{TorrentID: "pack-owner", Name: "pack", URL: "x", AnimeID: &anime.ID, Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end, EpisodeNumber: &ep, Status: model.DownloadStatusCompleted}
			s.db.Create(&pack)
			failed := model.Download{TorrentID: "failed", Name: "old failed episode", URL: "x", AnimeID: &anime.ID, EpisodeNumber: &ep, Status: model.DownloadStatusFailed, LastError: "original failure", SubtitleEvidence: `{"reason":"old evidence"}`}
			if scenario == "upgrade" {
				failed.Scope = model.DownloadScopeSeason
				failed.Purpose = model.DownloadPurposeSeasonUpgrade
			}
			s.db.Create(&failed)
			path := filepath.Join(s.cfg.MediaRoot, "episode.mkv")
			if scenario != "missing file" {
				if err := os.WriteFile(path, []byte("archived"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			owner := pack.TorrentID
			if scenario == "missing owner" {
				owner = "unrelated"
			}
			episode := model.AnimeEpisode{AnimeID: anime.ID, EpisodeNumber: ep, Downloaded: scenario != "unmapped", DownloadID: &owner, FilePath: &path}
			s.db.Create(&episode)
			s.resolvePriorFailures(pack.ID)
			s.resolveHistoricalFailures(context.Background())
			s.db.First(&failed, failed.ID)
			want := model.DownloadStatusFailed
			if scenario == "available" {
				want = model.DownloadStatusSuperseded
			}
			if failed.Status != want || failed.LastError != "original failure" || failed.SubtitleEvidence != `{"reason":"old evidence"}` {
				t.Fatal(failed)
			}
			if scenario != "available" {
				return
			}
			current, err := s.CurrentTasks(context.Background(), "all", "", 1, 100)
			if err != nil || current.Total != 1 || current.Counts["completed"] != 1 || current.Items[0].AttemptCount != 2 {
				t.Fatal(current, err)
			}
			history, err := s.AttemptHistory(context.Background(), pack.ID, 1, 1)
			if err != nil || history.Total != 2 || len(history.Items) != 1 {
				t.Fatal(history, err)
			}
			history, err = s.AttemptHistory(context.Background(), pack.ID, 2, 1)
			if err != nil || len(history.Items) != 1 {
				t.Fatal(history, err)
			}
			if content, err := os.ReadFile(path); err != nil || string(content) != "archived" {
				t.Fatal("archive changed", err)
			}
		})
	}
}

func TestBTCompletionQueueDoesNotBlockPatrolAndRestartDoesNotTransfer(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(fmt.Sprint("restart=", restart), func(t *testing.T) {
			s, row, result := stagedValidationFixture(t)
			e := &validationOnlyExecutor{}
			s.RegisterExecutor(model.DownloadTypeTorrent, e)
			if err := s.db.Model(&row).Updates(map[string]interface{}{"download_type": model.DownloadTypeTorrent, "source": SourceBT}).Error; err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			release := make(chan struct{})
			var calls atomic.Int32
			s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
				calls.Add(1)
				close(entered)
				<-release
				return mediacheck.SubtitleEvidence{Language: "simplified"}
			}
			if restart {
				s.RecoverPending(context.Background())
			} else {
				start := time.Now()
				if err := s.QueueEpisodeCompletion(context.Background(), row.ID, result.FilePath, result.FinalPath); err != nil {
					t.Fatal(err)
				}
				if time.Since(start) > time.Second {
					t.Fatal("patrol waited for OCR")
				}
			}
			<-entered
			if !restart {
				if err := s.QueueEpisodeCompletion(context.Background(), row.ID, result.FilePath, result.FinalPath); err != nil {
					t.Fatal(err)
				}
			}
			close(release)
			saved := waitValidationStatus(t, s, row.ID, model.DownloadStatusCompleted)
			if calls.Load() != 1 || e.calls.Load() != 0 || saved.ValidationPending {
				t.Fatal("BT transfer or OCR repeated", saved)
			}
		})
	}
}

func TestExplicitReviewCanRefreshUnknownEvidence(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	media, err := mediacheck.Probe(context.Background(), result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Reason: "unknown style"}
	}
	if _, err = s.inspectContent(context.Background(), &row, result.FilePath, media); err != nil {
		t.Fatal(err)
	}
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Language: "traditional"}
	}
	if e, err := s.inspectContent(context.Background(), &row, result.FilePath, media); err != nil || e.Language != "" {
		t.Fatal("automatic inspection retried unchanged unknown", e, err)
	}
	ctx := context.WithValue(context.Background(), refreshUnknownEvidence{}, true)
	if e, err := s.inspectContent(ctx, &row, result.FilePath, media); err != nil || e.Language != "traditional" {
		t.Fatal("explicit review could not improve prior unknown", e, err)
	}
	if e, err := s.inspectContent(context.Background(), &row, result.FilePath, media); err != nil || e.Language != "traditional" {
		t.Fatal("improved facts were not durable", e, err)
	}
}

func TestResumeCannotLoseAcceptanceBehindAnEndingPausedLease(t *testing.T) {
	s, row, _ := stagedValidationFixture(t)
	if err := s.db.Model(&row).Update("status", model.DownloadStatusPaused).Error; err != nil {
		t.Fatal(err)
	}
	_, cancel := context.WithCancel(context.Background())
	old := newValidationRun(cancel)
	s.validationCancels.Store(row.ID, old)
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Language: "simplified"}
	}
	if _, err := s.Resume(row.ID); err != nil {
		t.Fatal(err)
	}
	s.finishValidationRun(row.ID, old)
	got := waitValidationStatus(t, s, row.ID, model.DownloadStatusCompleted)
	if got.ValidationPending {
		t.Fatal("resume left completed payload stuck")
	}
}
