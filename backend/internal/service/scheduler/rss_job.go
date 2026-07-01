package scheduler

import (
	"context"

	"go.uber.org/zap"
)

// RSSRefresher is the narrow interface the RSS job depends on.
type RSSRefresher interface {
	RefreshAll(ctx context.Context)
}

// RSSEnabledFunc 在每次 Run 之前查询"是否启用 RSS 定时刷新与规则下载"。
// 返回 true 才会真正执行 RefreshAll；返回 false 直接跳过本轮。
// 传 nil 视为永远启用（向后兼容）。
type RSSEnabledFunc func(ctx context.Context) bool

type RSSRefreshJob struct {
	engine    RSSRefresher
	isEnabled RSSEnabledFunc
}

// NewRSSRefreshJob 构造 RSS 刷新 Job。
// isEnabled 为 nil 时等同于"永远启用"。
func NewRSSRefreshJob(engine RSSRefresher, isEnabled RSSEnabledFunc) *RSSRefreshJob {
	return &RSSRefreshJob{engine: engine, isEnabled: isEnabled}
}

func (j *RSSRefreshJob) Name() string { return "rss_refresh" }

func (j *RSSRefreshJob) Run(ctx context.Context) {
	if j.isEnabled != nil && !j.isEnabled(ctx) {
		zap.L().Debug("RSS 刷新已被全局开关关闭，跳过本轮")
		return
	}
	zap.L().Info("开始 RSS 刷新...")
	if j.engine != nil {
		j.engine.RefreshAll(ctx)
	}
	zap.L().Info("RSS 刷新完成")
}
