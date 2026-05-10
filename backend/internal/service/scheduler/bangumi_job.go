package scheduler

import (
	"context"

	"go.uber.org/zap"
)

// BangumiChecker 追番检查接口
type BangumiChecker interface {
	CheckAllSubscribed(ctx context.Context)
}

type BangumiCheckJob struct {
	checker BangumiChecker
}

func NewBangumiCheckJob(checker BangumiChecker) *BangumiCheckJob {
	return &BangumiCheckJob{checker: checker}
}

func (j *BangumiCheckJob) Name() string { return "bangumi_check" }

func (j *BangumiCheckJob) Run(ctx context.Context) {
	zap.L().Info("开始检查追番更新...")
	if j.checker != nil {
		j.checker.CheckAllSubscribed(ctx)
	}
	zap.L().Info("追番更新检查完成")
}
