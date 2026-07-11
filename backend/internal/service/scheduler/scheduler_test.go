package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

type mockJob struct {
	name  string
	count atomic.Int32
}

func (j *mockJob) Name() string { return j.name }
func (j *mockJob) Run(_ context.Context) {
	j.count.Add(1)
}

func TestScheduler_RegisterAndRun(t *testing.T) {
	sched := New()
	job := &mockJob{name: "test_job"}

	sched.Register(job, 50*time.Millisecond, true)
	sched.Start()
	defer sched.Stop()

	time.Sleep(120 * time.Millisecond)

	if job.count.Load() < 1 {
		t.Errorf("job ran %d times; expected at least 1", job.count.Load())
	}
}

func TestScheduler_Stop(t *testing.T) {
	sched := New()
	job := &mockJob{name: "stop_test"}

	sched.Register(job, 30*time.Millisecond, false)
	sched.Start()
	time.Sleep(50 * time.Millisecond)
	sched.Stop()

	countBefore := job.count.Load()
	time.Sleep(100 * time.Millisecond)
	countAfter := job.count.Load()

	if countAfter > countBefore+1 {
		t.Errorf("job kept running after Stop: before=%d after=%d", countBefore, countAfter)
	}
}

func TestScheduler_ImmediateRun(t *testing.T) {
	sched := New()
	job := &mockJob{name: "immediate"}

	sched.Register(job, 10*time.Second, true) // long interval, but run immediately
	sched.Start()
	defer sched.Stop()

	time.Sleep(50 * time.Millisecond)

	if job.count.Load() != 1 {
		t.Errorf("immediate run: count = %d; want 1", job.count.Load())
	}
}

func TestScheduler_NoImmediateRun(t *testing.T) {
	sched := New()
	job := &mockJob{name: "delayed"}

	sched.Register(job, 10*time.Second, false) // don't run immediately
	sched.Start()
	defer sched.Stop()

	time.Sleep(50 * time.Millisecond)

	if job.count.Load() != 0 {
		t.Errorf("no immediate run: count = %d; want 0", job.count.Load())
	}
}

func TestRSSRefreshJob(t *testing.T) {
	var called atomic.Bool
	mockEngine := &mockRSSRefresher{fn: func(ctx context.Context) { called.Store(true) }}

	job := NewRSSRefreshJob(mockEngine, nil)
	if job.Name() != "rss_refresh" {
		t.Errorf("Name = %q; want rss_refresh", job.Name())
	}

	job.Run(context.Background())
	if !called.Load() {
		t.Error("RefreshAll should have been called")
	}
}

type mockRSSRefresher struct {
	fn func(ctx context.Context)
}

func (m *mockRSSRefresher) RefreshAll(ctx context.Context) {
	if m.fn != nil {
		m.fn(ctx)
	}
}

func TestRSSRefreshJob_NilEngine(t *testing.T) {
	job := NewRSSRefreshJob(nil, nil)
	job.Run(context.Background()) // should not panic
}

func TestSelectRetryBatchDoesNotConsumeTruncatedRows(t *testing.T) {
	a1, a2, a3 := uint(1), uint(2), uint(3)
	rows := []model.Download{
		{ID: 11, AnimeID: &a1},
		{ID: 12, AnimeID: &a1},
		{ID: 21, AnimeID: &a2},
		{ID: 31, AnimeID: &a3},
	}

	animeIDs, rowIDs := selectRetryBatch(rows, 2)
	if len(animeIDs) != 2 || animeIDs[0] != 1 || animeIDs[1] != 2 {
		t.Fatalf("unexpected anime batch: %v", animeIDs)
	}
	if len(rowIDs) != 3 || rowIDs[0] != 11 || rowIDs[1] != 12 || rowIDs[2] != 21 {
		t.Fatalf("truncated anime rows must remain untouched, got %v", rowIDs)
	}
}
