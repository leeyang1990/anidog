package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// commitUpgradeJournal is the common file/DB decision boundary for verified
// replacements. Callers define the affected assets; interruption, rollback,
// ambiguous commit acknowledgement and startup replay share one mechanism.
// Caller holds completionMu, and validates all incoming assets before entry.
func (s *Service) commitUpgradeJournal(ctx context.Context, candidate *model.Download, journalPath string, j *upgradeJournal, apply func(*gorm.DB) error) bool {
	if err := checkUpgradePaths(s.currentMediaRoot(ctx), append([]upgradeMove{{journalPath, journalPath}}, j.Moves...)); err != nil {
		s.holdUpgrade(candidate.ID, err)
		return false
	}
	if err := writeUpgradeJournal(journalPath, j); err != nil {
		s.holdUpgrade(candidate.ID, err)
		return false
	}
	fail := func(err error) bool {
		rollbackErr := rollbackUpgrade(j)
		if rollbackErr == nil {
			j.State = "rolled_back"
			_ = writeUpgradeJournal(journalPath, j)
		}
		note := fmt.Sprintf("媒体升级切换失败，原版已保留: %v", err)
		if rollbackErr != nil {
			note = fmt.Sprintf("媒体升级需恢复，已暂停: %v；回滚: %v", err, rollbackErr)
		}
		// A successful rollback does not resolve the cause of the failed swap.
		// Pause both outcomes so the BT poller cannot repeat this every few seconds.
		s.db.Model(&model.Download{}).Where("id = ? AND status IN ?", candidate.ID, []string{model.DownloadStatusPending, model.DownloadStatusDownloading}).Updates(map[string]interface{}{"status": model.DownloadStatusPaused, "last_error": note})
		zap.L().Error(note)
		return false
	}
	// Stop peer reads and writes before moving any candidate files. With classic
	// file IO their handles close at the end of each operation, including on SMB.
	if executor := s.executors[candidate.DownloadType]; executor != nil {
		if err := executor.Pause(executorTaskID(candidate)); err != nil {
			return fail(fmt.Errorf("暂停升级候选的文件访问: %w", err))
		}
	}
	for _, m := range j.Moves {
		if _, err := os.Stat(m.To); err == nil {
			return fail(fmt.Errorf("升级目标已存在: %s", m.To))
		} else if !os.IsNotExist(err) {
			return fail(err)
		}
		if err := os.MkdirAll(filepath.Dir(m.To), 0755); err != nil {
			return fail(err)
		}
		if err := os.Rename(m.From, m.To); err != nil {
			return fail(err)
		}
	}

	j.State = "committing"
	if err := writeUpgradeJournal(journalPath, j); err != nil {
		return fail(err)
	}
	readyToCommit := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := apply(tx)
		readyToCommit = err == nil
		return err
	})

	if err != nil {
		if !readyToCommit {
			return fail(err)
		}
		// The request may have been cancelled after the server committed. Resolve
		// independently of that request, with a bounded primary database read.
		resolveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		committed, resolveErr := s.upgradeCommitted(resolveCtx, j)
		cancel()
		if resolveErr != nil {
			// Keep both copies and the nonterminal journal. In particular, never
			// overwrite a possibly committed task or archive this as rolled back.
			zap.L().Error("媒体升级提交结果待恢复；保留新旧文件和恢复日志", zap.Uint("id", candidate.ID), zap.Error(err), zap.NamedError("resolution", resolveErr))
			return false
		}
		if !committed {
			return fail(err)
		}
	}
	j.State = "committed"
	if err = writeUpgradeJournal(journalPath, j); err != nil {
		zap.L().Warn("媒体升级已提交，启动时将补齐日志", zap.Error(err))
	}
	return true
}

func (s *Service) holdUpgrade(id uint, err error) {
	s.updateStatus(id, model.DownloadStatusPaused, map[string]interface{}{"last_error": fmt.Sprintf("媒体升级暂停，保留原版与候选：%v", err)})
}

// Resolve the nearest existing ancestor, including dangling symlinks. Lexical
// containment alone cannot protect a new destination under a linked directory.
func checkUpgradePaths(root string, moves []upgradeMove) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for _, move := range moves {
		for _, path := range []string{move.From, move.To} {
			if !pathWithin(path, root) {
				return fmt.Errorf("升级路径越界: %s", path)
			}
			for parent := filepath.Clean(path); ; parent = filepath.Dir(parent) {
				_, err := os.Lstat(parent)
				if os.IsNotExist(err) && parent != filepath.Clean(root) {
					continue
				}
				if err != nil {
					return err
				}
				resolved, err := filepath.EvalSymlinks(parent)
				if err != nil {
					return err
				}
				if !pathWithin(resolved, resolvedRoot) {
					return fmt.Errorf("升级路径指向媒体目录之外: %s", path)
				}
				break
			}
		}
	}
	return nil
}
