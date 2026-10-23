package download

import (
	"context"
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
	"github.com/anidog/anidog-go/internal/service/stream"
	"github.com/shirou/gopsutil/v3/disk"
)

type validationOnlyExecutor struct{ calls atomic.Int32 }

func (e *validationOnlyExecutor) Execute(context.Context, *Task, ProgressCallback) (*Result, error) {
	e.calls.Add(1)
	return nil, fmt.Errorf("unexpected playback transfer")
}
func (e *validationOnlyExecutor) Cancel(string) error       { return nil }
func (e *validationOnlyExecutor) Pause(string) error        { return nil }
func (e *validationOnlyExecutor) Resume(string) error       { return nil }
func (e *validationOnlyExecutor) Remove(string, bool) error { return nil }

func stagedValidationFixture(t *testing.T) (*Service, model.Download, *Result) {
	t.Helper()
	s := capacityService(t, 500*gib)
	s.cfg.VerifyDownloadedMedia = true
	anime := model.Anime{Title: "Local proof", IsSubscribed: true}
	if err := s.db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	ep := 1
	path := filepath.Join(s.cfg.MediaRoot, "episode.anidog-check.mp4")
	final := filepath.Join(s.cfg.MediaRoot, "episode.mp4")
	if err := os.WriteFile(path, upgradeTestVideo(t), 0600); err != nil {
		t.Fatal(err)
	}
	row := model.Download{TorrentID: "local-proof", Name: "Local proof", URL: "https://expired.invalid/episode", Source: SourceStream, SourceTitle: "[CHS] catalog claim", DownloadType: model.DownloadTypeStream, Status: model.DownloadStatusDownloading, AnimeID: &anime.ID, EpisodeNumber: &ep}
	if err := s.db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	result := &Result{FilePath: path, FinalPath: final}
	if err := s.retainStreamResult(context.Background(), row.ID, result); err != nil {
		t.Fatal(err)
	}
	if err := s.db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	return s, row, result
}

func waitValidationStatus(t *testing.T, s *Service, id uint, status string) model.Download {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var row model.Download
		if err := s.db.First(&row, id).Error; err != nil {
			t.Fatal(err)
		}
		if row.Status == status {
			return row
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("download %d did not reach %s", id, status)
	return model.Download{}
}

func TestUnknownStreamSubtitlesDeferProofToLocalPayload(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		t.Error("preflight must not seek remote subtitle frames")
		return mediacheck.SubtitleEvidence{}
	}
	s.db.Model(&row).Update("verified_languages", `["traditional"]`)
	task := validationTask(&row)
	if err := s.streamPreflight(row.ID, task)(context.Background(), &stream.InterceptedVideo{URL: result.FilePath}); err != nil {
		t.Fatal(err)
	}
	s.db.First(&row, row.ID)
	if row.VerifiedLanguages != "" || row.TotalBytes == nil || row.Status == model.DownloadStatusCompleted {
		t.Fatalf("unknown subtitles granted acceptance: %+v", row)
	}
}

func TestLocalStreamAcceptanceRequiresActualSubtitleProof(t *testing.T) {
	for _, tc := range []struct {
		name, language string
		retryable      bool
		wantCode       failure.Code
		completed      bool
	}{
		{name: "simplified", language: "simplified", completed: true},
		{name: "traditional", language: "traditional", wantCode: failure.PreferenceMismatch},
		{name: "unknown despite catalog CHS", wantCode: failure.PreferenceMismatch},
		{name: "inspection outage", retryable: true, wantCode: failure.InspectionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, row, result := stagedValidationFixture(t)
			s.cfg.VerifyDownloadedMedia = false // Automatic stream preferences remain mandatory.
			s.subtitleInspector = func(_ context.Context, path string, args []string, _ float64) mediacheck.SubtitleEvidence {
				if path != result.FilePath || len(args) != 0 {
					t.Error("OCR did not inspect the isolated local payload")
				}
				return mediacheck.SubtitleEvidence{Language: tc.language, Retryable: tc.retryable, Reason: "fixture evidence"}
			}
			if won := s.CompleteEpisodeRace(context.Background(), row.ID, result.FilePath, result.FinalPath); won != tc.completed {
				t.Fatalf("won=%v", won)
			}
			s.db.First(&row, row.ID)
			if tc.completed {
				if row.ValidationPending || row.VerifiedLanguages != `["simplified"]` || row.Status != model.DownloadStatusCompleted {
					t.Fatalf("bad acceptance: %+v", row)
				}
			} else {
				if row.Status != model.DownloadStatusFailed || row.FailureCode != string(tc.wantCode) || row.ValidationPending != tc.retryable || row.FilePath == nil || *row.FilePath != result.FilePath {
					t.Fatalf("lost failure or payload: %+v", row)
				}
				if _, err := os.Stat(result.FilePath); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(result.FinalPath); !os.IsNotExist(err) {
					t.Fatal("unaccepted media entered library")
				}
				var count int64
				s.db.Model(&model.AnimeEpisode{}).Where("downloaded = ?", true).Count(&count)
				if count != 0 {
					t.Fatal("rejected media projected as playable")
				}
			}
		})
	}
}

func TestManualRetryRevalidatesRetainedPayloadWithoutRuleOrCapacity(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	e := &validationOnlyExecutor{}
	s.RegisterExecutor(model.DownloadTypeStream, e)
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Language: "simplified"}
	}
	s.db.Model(&row).Updates(map[string]interface{}{"status": model.DownloadStatusFailed, "failure_code": string(failure.InspectionFailed), "failure_stage": string(failure.Subtitles), "failure_kind": model.FailureKindTransient})
	s.capacityProbe = func(string) (*disk.UsageStat, error) {
		t.Error("retained validation reserved another payload")
		return nil, fmt.Errorf("no space")
	}
	if _, err := s.Retry(context.Background(), row.ID); err != nil {
		t.Fatal(err)
	}
	saved := waitValidationStatus(t, s, row.ID, model.DownloadStatusCompleted)
	if e.calls.Load() != 0 || saved.ValidationPending || saved.FilePath == nil || *saved.FilePath != result.FinalPath {
		t.Fatal("retry fetched playback or lost promotion")
	}
}

func TestAutomaticValidationClaimsOnceAndHonorsBudgetAndCancel(t *testing.T) {
	s, row, _ := stagedValidationFixture(t)
	e := &validationOnlyExecutor{}
	s.RegisterExecutor(model.DownloadTypeStream, e)
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s.subtitleInspector = func(ctx context.Context, _ string, _ []string, _ float64) mediacheck.SubtitleEvidence {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return mediacheck.SubtitleEvidence{Language: "simplified"}
		case <-ctx.Done():
			return mediacheck.SubtitleEvidence{Retryable: true}
		}
	}
	s.db.Model(&row).Updates(map[string]interface{}{"status": model.DownloadStatusFailed, "failure_kind": model.FailureKindTransient, "failure_code": string(failure.InspectionFailed), "failure_stage": string(failure.Subtitles)})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.ResumeMediaValidation(context.Background(), *row.AnimeID) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("local validation did not start")
	}
	if err := s.Cancel(row.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	saved := waitValidationStatus(t, s, row.ID, model.DownloadStatusFailed)
	if saved.FailureCode != string(failure.Canceled) || saved.RetryCount != 1 || e.calls.Load() != 0 {
		t.Fatalf("cancel or bounded claim lost: %+v", saved)
	}
	if err := s.ResumeMediaValidation(context.Background(), *row.AnimeID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	s.db.First(&saved, row.ID)
	if saved.Status != model.DownloadStatusFailed || calls.Load() != 1 {
		t.Fatal("automatic recovery resurrected explicit cancel")
	}
	s.db.Model(&saved).Updates(map[string]interface{}{"failure_code": string(failure.InspectionFailed), "retry_count": 3})
	if err := s.ResumeMediaValidation(context.Background(), *row.AnimeID); err != nil {
		t.Fatal(err)
	}
	s.db.First(&saved, row.ID)
	if saved.Status != model.DownloadStatusFailed || saved.RetryCount != 3 {
		t.Fatal("automatic validation reset exhausted budget")
	}
}

func TestRestartResumesRetainedAcceptanceAndRenameCommitGap(t *testing.T) {
	for _, renamed := range []bool{false, true} {
		t.Run(fmt.Sprint("already_renamed=", renamed), func(t *testing.T) {
			s, row, result := stagedValidationFixture(t)
			e := &validationOnlyExecutor{}
			s.RegisterExecutor(model.DownloadTypeStream, e)
			s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
				return mediacheck.SubtitleEvidence{Language: "simplified"}
			}
			if renamed {
				if err := os.Rename(result.FilePath, result.FinalPath); err != nil {
					t.Fatal(err)
				}
			}
			s.RecoverPending(context.Background())
			saved := waitValidationStatus(t, s, row.ID, model.DownloadStatusCompleted)
			if e.calls.Load() != 0 || saved.ValidationPending || saved.FilePath == nil || *saved.FilePath != result.FinalPath {
				t.Fatal("restart did not resume local acceptance")
			}
		})
	}
}

func TestRetainedPayloadMissingAndSymlinkDoNotStartAnotherTransfer(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(fmt.Sprint("symlink=", symlink), func(t *testing.T) {
			s, row, result := stagedValidationFixture(t)
			e := &validationOnlyExecutor{}
			s.RegisterExecutor(model.DownloadTypeStream, e)
			if err := os.Remove(result.FilePath); err != nil {
				t.Fatal(err)
			}
			if symlink {
				outside := filepath.Join(t.TempDir(), "outside.mp4")
				if err := os.WriteFile(outside, []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, result.FilePath); err != nil {
					t.Skip(err)
				}
				if _, err := s.retainedValidationResult(&row); err == nil {
					t.Fatal("retained symlink accepted")
				}
			} else {
				s.db.Model(&row).Updates(map[string]interface{}{"status": model.DownloadStatusFailed, "failure_kind": model.FailureKindTransient, "failure_code": string(failure.InspectionFailed)})
				if err := s.ResumeMediaValidation(context.Background(), *row.AnimeID); err != nil {
					t.Fatal(err)
				}
				s.db.First(&row, row.ID)
				if row.ValidationPending || row.Status != model.DownloadStatusFailed || e.calls.Load() != 0 {
					t.Fatal("missing payload was blindly reexecuted")
				}
			}
		})
	}
}

func TestSlowSubtitleInspectionDoesNotHoldGlobalCompletionLock(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		close(entered)
		<-release
		return mediacheck.SubtitleEvidence{Language: "simplified"}
	}
	go func() {
		defer close(done)
		s.CompleteEpisodeRace(context.Background(), row.ID, result.FilePath, result.FinalPath)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("inspection did not start")
	}
	ep := 2
	other := model.Download{TorrentID: "independent", Name: "independent", URL: "fixture", Source: SourceManual, DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusDownloading, AnimeID: row.AnimeID, EpisodeNumber: &ep}
	if err := s.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.cfg.MediaRoot, "other-stage.mp4")
	final := filepath.Join(s.cfg.MediaRoot, "other.mp4")
	if err := os.WriteFile(path, upgradeTestVideo(t), 0600); err != nil {
		t.Fatal(err)
	}
	independent := make(chan bool, 1)
	go func() { independent <- s.CompleteEpisodeRace(context.Background(), other.ID, path, final) }()
	select {
	case won := <-independent:
		if !won {
			t.Error("independent completion failed")
		}
	case <-time.After(3 * time.Second):
		t.Error("subtitle inspection blocked another completion")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream completion did not finish")
	}
}

func TestRestartDoesNotBorrowOldLanguageProof(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	e := &validationOnlyExecutor{}
	s.RegisterExecutor(model.DownloadTypeStream, e)
	s.db.Model(&row).Update("verified_languages", `["simplified"]`)
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		return mediacheck.SubtitleEvidence{Language: "traditional"}
	}
	s.RecoverPending(context.Background())
	saved := waitValidationStatus(t, s, row.ID, model.DownloadStatusFailed)
	if saved.FailureCode != string(failure.PreferenceMismatch) || e.calls.Load() != 0 {
		t.Fatalf("stale OCR proof accepted: %+v", saved)
	}
	if _, err := os.Stat(result.FinalPath); !os.IsNotExist(err) {
		t.Fatal("unverified replacement entered library")
	}
}

func TestQueuedValidationCanBeCanceledBeforeInspection(t *testing.T) {
	s, row, result := stagedValidationFixture(t)
	e := &validationOnlyExecutor{}
	s.RegisterExecutor(model.DownloadTypeStream, e)
	s.validationSlots = make(chan struct{}, 1)
	s.validationSlots <- struct{}{}
	var inspections atomic.Int32
	s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
		inspections.Add(1)
		return mediacheck.SubtitleEvidence{Language: "simplified"}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.finishStreamResult(context.Background(), row.ID, validationTask(&row), result)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := s.validationCancels.Load(row.ID); ok {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := s.Cancel(row.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("queued validation ignored cancel")
	}
	<-s.validationSlots
	if inspections.Load() != 0 {
		t.Fatal("canceled queued work ran OCR")
	}
}
