package download

import (
	"context"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

type retryCaptureExecutor struct {
	pathCaptureExecutor
	tasks chan Task
}

func (e *retryCaptureExecutor) Execute(_ context.Context, task *Task, _ ProgressCallback) (*Result, error) {
	e.tasks <- *task
	return &Result{TorrentID: testTorrentReceipt(task)}, nil
}

func TestExplicitRetryResetsEvidenceAndActuallyRequeuesTorrent(t *testing.T) {
	s := capacityService(t, 500*gib)
	executor := &retryCaptureExecutor{tasks: make(chan Task, 2)}
	s.RegisterExecutor(model.DownloadTypeTorrent, executor)
	start, end := 1, 12
	next := time.Now().Add(24 * time.Hour)
	row := model.Download{TorrentID: "retry", Name: "retry", URL: "magnet:retry", Source: SourceBT, SourceTitle: "original", DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusFailed, FailureKind: model.FailureKindExhausted, FailureCode: "inspection_failed", FailureStage: "subtitles", RetryCount: 3, NextRetryAt: &next, LastError: "old failure", VerifiedLanguages: `["traditional"]`, Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end}
	if err := s.db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	result, err := s.Retry(context.Background(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.RetryCount != 0 || result.FailureKind != "" || result.FailureCode != "" || result.FailureStage != "" || result.NextRetryAt != nil || result.VerifiedLanguages != "" {
		t.Fatal("explicit retry retained stale failure/evidence", result)
	}
	select {
	case task := <-executor.tasks:
		if task.Scope != model.DownloadScopeSeason || task.EpisodeStart == nil || *task.EpisodeStart != 1 || *task.EpisodeEnd != 12 || task.Source != SourceBT || task.RetryCount != 0 || task.SourceTitle != "original" {
			t.Fatal("retried task lost original scope or policy", task)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("torrent retry changed the DB but never called its executor")
	}
	if _, err := s.Retry(context.Background(), row.ID); err == nil {
		t.Fatal("duplicate user retry admitted")
	}
}

func TestExplicitRetryCannotResurrectPendingTorrentRemoval(t *testing.T) {
	s := capacityService(t, 500*gib)
	row := model.Download{TorrentID: "cleaning", Name: "cleaning", URL: "magnet:cleaning", DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusFailed, TorrentRemovalPending: true}
	s.db.Create(&row)
	if _, err := s.Retry(context.Background(), row.ID); err == nil {
		t.Fatal("pending cleanup was resurrected as an active torrent")
	}
	var saved model.Download
	s.db.First(&saved, row.ID)
	if saved.Status != model.DownloadStatusFailed || !saved.TorrentRemovalPending {
		t.Fatal(saved)
	}
}
