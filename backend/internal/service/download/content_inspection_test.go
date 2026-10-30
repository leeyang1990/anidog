package download

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"gorm.io/gorm"
)

func TestCancelledContentInspectionDoesNotFailAnotherAttempt(t *testing.T) {
	s, a, result := stagedValidationFixture(t)
	media, err := mediacheck.Probe(context.Background(), result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	b := a
	b.ID = 0
	b.TorrentID = "independent-source"
	if err = s.db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	s.videoHasher = func(context.Context, string, string) (string, error) { return "same-video", nil }
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		s.subtitleInspector = func(ctx context.Context, _ string, _ []string, _ float64) mediacheck.SubtitleEvidence {
			if calls.Add(1) == 1 {
				<-ctx.Done()
				return mediacheck.SubtitleEvidence{Retryable: true, Reason: "first attempt canceled"}
			}
			return mediacheck.SubtitleEvidence{Language: "simplified"}
		}
		ctx, cancel := context.WithCancel(context.Background())
		first := make(chan error, 1)
		go func() { _, e := s.inspectContent(ctx, &a, result.FilePath, media); first <- e }()
		synctest.Wait()
		second := make(chan error, 1)
		go func() {
			e, err := s.inspectContent(context.Background(), &b, result.FilePath, media)
			if err == nil && (e.Language != "simplified" || e.Retryable) {
				err = errors.New("independent attempt inherited another caller's canceled inspection")
			}
			second <- err
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		if err := <-first; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if err := <-second; err != nil {
			t.Error(err)
		}
		if calls.Load() != 2 {
			t.Error("remaining attempt did not execute its inspection", calls.Load())
		}
	})
}

func TestExplicitContentRefreshDoesNotReuseConcurrentUnknown(t *testing.T) {
	s, a, result := stagedValidationFixture(t)
	media, err := mediacheck.Probe(context.Background(), result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	b := a
	b.ID = 0
	b.TorrentID = "refresh-source"
	if err = s.db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	s.videoHasher = func(context.Context, string, string) (string, error) { return "same-video", nil }
	if err = s.db.Create(&model.MediaEvidence{VideoHash: "same-video", InspectorVersion: mediacheck.SubtitleInspectorVersion, Evidence: `{"inspector_version":"subtitle-v4","language":"","reason":"cached unknown"}`, OriginDownloadID: &a.ID}).Error; err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		var lookups, calls atomic.Int32
		if err := s.db.Callback().Query().After("gorm:query").Register("followup_hold_cached_read", func(tx *gorm.DB) {
			if tx.Statement.Table == "media_evidence" && lookups.Add(1) == 1 {
				<-release
			}
		}); err != nil {
			t.Fatal(err)
		}
		defer s.db.Callback().Query().Remove("followup_hold_cached_read")
		s.subtitleInspector = func(context.Context, string, []string, float64) mediacheck.SubtitleEvidence {
			calls.Add(1)
			return mediacheck.SubtitleEvidence{Language: "simplified"}
		}
		first := make(chan error, 1)
		go func() { _, e := s.inspectContent(context.Background(), &a, result.FilePath, media); first <- e }()
		synctest.Wait()
		second := make(chan error, 1)
		go func() {
			ctx := context.WithValue(context.Background(), refreshUnknownEvidence{}, true)
			e, err := s.inspectContent(ctx, &b, result.FilePath, media)
			if err == nil && e.Language != "simplified" {
				err = errors.New("explicit refresh silently accepted another request's cached unknown")
			}
			second <- err
		}()
		synctest.Wait()
		close(release)
		synctest.Wait()
		if err := <-first; err != nil {
			t.Fatal(err)
		}
		if err := <-second; err != nil {
			t.Error(err)
		}
		if calls.Load() != 1 {
			t.Error("explicit refresh performed no measurement", calls.Load())
		}
	})
}

func TestSharedInspectionPublishesOnlySuitableResults(t *testing.T) {
	for _, tc := range []struct {
		name              string
		evidence          mediacheck.SubtitleEvidence
		err               error
		measured, refresh bool
		executions        int
	}{
		{name: "transient failure", evidence: mediacheck.SubtitleEvidence{Retryable: true}, executions: 2},
		{name: "execution error", err: errors.New("owner file unavailable"), executions: 2},
		{name: "fresh unknown satisfies refresh", evidence: mediacheck.SubtitleEvidence{}, measured: true, refresh: true, executions: 1},
		{name: "cached known satisfies refresh", evidence: mediacheck.SubtitleEvidence{Language: "traditional"}, refresh: true, executions: 1},
		{name: "successful inspection remains shared", evidence: mediacheck.SubtitleEvidence{Language: "simplified"}, measured: true, executions: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				group := &contentInspectionGroup{}
				release := make(chan struct{})
				var calls atomic.Int32
				first := make(chan error, 1)
				go func() {
					_, e := group.inspect(context.Background(), "video", false, func() (contentInspection, error) {
						calls.Add(1)
						<-release
						return contentInspection{evidence: tc.evidence, measured: tc.measured}, tc.err
					})
					first <- e
				}()
				synctest.Wait()
				second := make(chan contentInspection, 1)
				secondErr := make(chan error, 1)
				go func() {
					ctx := context.WithValue(context.Background(), refreshUnknownEvidence{}, tc.refresh)
					e, err := group.inspect(ctx, "video", tc.refresh, func() (contentInspection, error) {
						calls.Add(1)
						return contentInspection{evidence: mediacheck.SubtitleEvidence{Language: "simplified"}, measured: true}, nil
					})
					second <- e
					secondErr <- err
				}()
				synctest.Wait()
				close(release)
				synctest.Wait()
				if !errors.Is(<-first, tc.err) {
					t.Error("owner failure lost")
				}
				if err := <-secondErr; err != nil {
					t.Fatal(err)
				}
				if got := <-second; got.evidence.Retryable {
					t.Fatal("another attempt's failure was reused")
				}
				if calls.Load() != int32(tc.executions) {
					t.Fatal(calls.Load(), tc.executions)
				}
			})
		})
	}
}

func TestCancellingFollowerDoesNotInterruptMeasurement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		group := &contentInspectionGroup{}
		release := make(chan struct{})
		var calls atomic.Int32
		first := make(chan error, 1)
		go func() {
			_, e := group.inspect(context.Background(), "video", false, func() (contentInspection, error) {
				calls.Add(1)
				<-release
				return contentInspection{evidence: mediacheck.SubtitleEvidence{Language: "simplified"}, measured: true}, nil
			})
			first <- e
		}()
		synctest.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		second := make(chan error, 1)
		go func() {
			_, e := group.inspect(ctx, "video", false, func() (contentInspection, error) {
				t.Error("unneeded follower execution")
				return contentInspection{}, nil
			})
			second <- e
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		if !errors.Is(<-second, context.Canceled) {
			t.Fatal("follower cancellation lost")
		}
		close(release)
		synctest.Wait()
		if err := <-first; err != nil || calls.Load() != 1 {
			t.Fatal("owner was interrupted", err, calls.Load())
		}
	})
}

func TestCancelledSourceDoesNotPreventRetainedPeerFromArchiving(t *testing.T) {
	s, a, result := stagedValidationFixture(t)
	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(s.cfg.MediaRoot, "second.anidog-check.mp4")
	if err = os.WriteFile(secondPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	b := a
	b.ID = 0
	b.TorrentID = "parallel-proof"
	b.URL = "https://alternate.invalid/episode"
	b.FilePath = &secondPath
	if err = s.db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	media, err := mediacheck.Probe(context.Background(), result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := mediacheck.VideoDigest(context.Background(), result.FilePath, media.Streams[0].CodecName)
	if err != nil {
		t.Fatal(err)
	}
	s.videoHasher = func(context.Context, string, string) (string, error) { return digest, nil }
	executor := &validationOnlyExecutor{}
	s.RegisterExecutor(model.DownloadTypeStream, executor)
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		s.subtitleInspector = func(ctx context.Context, _ string, _ []string, _ float64) mediacheck.SubtitleEvidence {
			if calls.Add(1) == 1 {
				<-ctx.Done()
				return mediacheck.SubtitleEvidence{Retryable: true, Reason: "canceled source"}
			}
			return mediacheck.SubtitleEvidence{Language: "simplified"}
		}
		if err := s.QueueEpisodeCompletion(context.Background(), a.ID, result.FilePath, result.FinalPath); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if err := s.QueueEpisodeCompletion(context.Background(), b.ID, secondPath, result.FinalPath); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if err := s.Cancel(a.ID); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		var saved model.Download
		if err := s.db.First(&saved, b.ID).Error; err != nil {
			t.Fatal(err)
		}
		if saved.Status != model.DownloadStatusCompleted || saved.ValidationPending || calls.Load() != 2 || executor.calls.Load() != 0 {
			t.Fatal("qualified retained peer did not archive without a new transfer", saved.Status, saved.LastError, calls.Load(), executor.calls.Load())
		}
		actual, err := os.ReadFile(result.FinalPath)
		if err != nil || !bytes.Equal(actual, data) {
			t.Fatal("archived payload differs", err)
		}
		var evidence model.MediaEvidence
		if err := s.db.First(&evidence).Error; err != nil || evidence.OriginDownloadID == nil || *evidence.OriginDownloadID != b.ID {
			t.Fatal("wrong measurement source", evidence, err)
		}
	})
}
