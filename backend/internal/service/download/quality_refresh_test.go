package download

import (
	"context"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
	"sync/atomic"
	"testing"
	"time"
)

func TestQualityRefreshCoalescesButKeepsChangesDuringAudit(t *testing.T) {
	s := &Service{}
	entered, release, done := make(chan int, 4), make(chan struct{}), make(chan struct{})
	var count atomic.Int32
	s.SetMediaCompletedHandler(func(ctx context.Context, id uint) error {
		if _, ok := ctx.Deadline(); !ok || id != 7 {
			t.Error("audit lacks identity or deadline")
		}
		n := int(count.Add(1))
		entered <- n
		if n == 1 {
			<-release
		} else {
			close(done)
		}
		return nil
	})
	s.requestQualityRefresh(7)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("audit not started")
	}
	for i := 0; i < 100; i++ {
		s.requestQualityRefresh(7)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("change during audit was lost")
	}
	if count.Load() != 2 {
		t.Fatal("completion storm did not coalesce", count.Load())
	}
}

func TestNewAssetInvalidatesQualityButDuplicateCompletionKeepsIt(t *testing.T) {
	s := capacityService(t, 500*gib)
	a := model.Anime{Title: "asset"}
	if err := s.db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	ep, oldID, newID, path := 1, "old", "new", "/same/final/path.mp4"
	now := time.Now()
	row := model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: ep, Downloaded: true, DownloadID: &oldID, FilePath: &path, QualityStatus: model.QualityMeetsTarget, QualityFingerprint: "old", QualityCheckedAt: &now}
	if err := s.db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	d := model.Download{TorrentID: newID, Status: model.DownloadStatusCompleted, AnimeID: &a.ID, EpisodeNumber: &ep, FilePath: &path}
	if err := s.syncCompletedEpisodeState(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	row = model.AnimeEpisode{ID: row.ID}
	if err := s.db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.QualityStatus != model.QualityUnknown || row.QualityFingerprint != "" || row.QualityCheckedAt != nil {
		t.Fatalf("old proof leaked to replacement: %+v", row)
	}
	if err := s.db.Model(&row).Updates(map[string]interface{}{"quality_status": model.QualityMeetsTarget, "quality_fingerprint": "fresh", "quality_checked_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.syncCompletedEpisodeState(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	row = model.AnimeEpisode{ID: row.ID}
	if err := s.db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.QualityStatus != model.QualityMeetsTarget || row.QualityFingerprint != "fresh" {
		t.Fatal("duplicate callback invalidated unchanged media", row)
	}
}

func TestInspectionFailurePersistsFactsWithoutSourceOutage(t *testing.T) {
	err := failure.Wrap(failure.Subtitles, failure.InspectionFailed, context.DeadlineExceeded)
	fields, kind, delay := failureFields(err, 3)
	if kind != model.FailureKindExhausted || delay != 0 || fields["next_retry_at"] != nil {
		t.Fatal(fields)
	}
	row := model.Download{FailureKind: kind, FailureCode: fields["failure_code"].(string), FailureStage: fields["failure_stage"].(string), LastError: "renamed"}
	if failure.FromRecord(row).AvailabilityFailure {
		t.Fatal("exhausted OCR poisoned source health")
	}
}
