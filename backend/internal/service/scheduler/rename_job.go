package scheduler

import (
	"context"

	"go.uber.org/zap"
)

type RenameJob struct{}

func NewRenameJob() *RenameJob { return &RenameJob{} }

func (j *RenameJob) Name() string { return "rename" }

func (j *RenameJob) Run(ctx context.Context) {
	// TODO: call Renamer.Rename
	zap.L().Debug("重命名任务执行（占位）")
}
