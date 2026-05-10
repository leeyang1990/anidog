package scheduler

import (
	"context"

	"go.uber.org/zap"
)

// RSSRefresher is the narrow interface the RSS job depends on.
type RSSRefresher interface {
	RefreshAll(ctx context.Context)
}

type RSSRefreshJob struct {
	engine RSSRefresher
}

func NewRSSRefreshJob(engine RSSRefresher) *RSSRefreshJob {
	return &RSSRefreshJob{engine: engine}
}

func (j *RSSRefreshJob) Name() string { return "rss_refresh" }

func (j *RSSRefreshJob) Run(ctx context.Context) {
	zap.L().Info("开始 RSS 刷新...")
	if j.engine != nil {
		j.engine.RefreshAll(ctx)
	}
	zap.L().Info("RSS 刷新完成")
}
