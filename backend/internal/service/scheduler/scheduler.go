package scheduler

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Job is a unit of work that the scheduler runs periodically.
type Job interface {
	Name() string
	Run(ctx context.Context)
}

type scheduledJob struct {
	job          Job
	interval     time.Duration
	runImmediate bool
}

// Scheduler runs registered Jobs at their specified intervals.
type Scheduler struct {
	jobs   []scheduledJob
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New() *Scheduler {
	return &Scheduler{}
}

// Register adds a job to be run at the given interval.
// If runImmediate is true, the job runs once immediately on Start.
func (s *Scheduler) Register(job Job, interval time.Duration, runImmediate bool) {
	s.jobs = append(s.jobs, scheduledJob{job: job, interval: interval, runImmediate: runImmediate})
}

func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	for _, sj := range s.jobs {
		s.wg.Add(1)
		go s.runLoop(ctx, sj)
	}

	if len(s.jobs) > 0 {
		zap.L().Info("调度器已启动", zap.Int("jobs", len(s.jobs)))
	} else {
		zap.L().Info("调度器已启动（无任务）")
	}
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	zap.L().Info("调度器已停止")
}

func (s *Scheduler) runLoop(ctx context.Context, sj scheduledJob) {
	defer s.wg.Done()

	if sj.runImmediate {
		sj.job.Run(ctx)
	}

	ticker := time.NewTicker(sj.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sj.job.Run(ctx)
		}
	}
}
