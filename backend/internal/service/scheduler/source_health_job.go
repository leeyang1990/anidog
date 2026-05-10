package scheduler

import (
	"context"

	"go.uber.org/zap"
)

// SourceHealthChecker 源健康检测接口
type SourceHealthChecker interface {
	CheckAllSubscribed(ctx context.Context)
}

type SourceHealthJob struct {
	checker SourceHealthChecker
}

func NewSourceHealthJob(checker SourceHealthChecker) *SourceHealthJob {
	return &SourceHealthJob{checker: checker}
}

func (j *SourceHealthJob) Name() string { return "source_health" }

func (j *SourceHealthJob) Run(ctx context.Context) {
	zap.L().Debug("开始源健康检测...")
	if j.checker != nil {
		j.checker.CheckAllSubscribed(ctx)
	}
}
