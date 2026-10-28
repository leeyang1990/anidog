package download

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/episode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// episodeUpgradeState distinguishes confirmed quality debt from availability.
// Ordinary race winners and unknown quality never authorize replacing media.
func (s *Service) episodeUpgradeState(ctx context.Context, candidate *model.Download) (*model.AnimeEpisode, error) {
	if candidate.Status != model.DownloadStatusPending && candidate.Status != model.DownloadStatusDownloading || candidate.AnimeID == nil || candidate.EpisodeNumber == nil || candidate.Scope == model.DownloadScopeSeason || s.cfg == nil {
		return nil, nil
	}
	var anime model.Anime
	var episodes []model.AnimeEpisode
	if err := s.db.WithContext(ctx).First(&anime, *candidate.AnimeID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("anime_id = ?", anime.ID).Find(&episodes).Error; err != nil {
		return nil, err
	}
	if !episode.SeasonFinished(&anime, episodes, time.Now()) {
		return nil, nil
	}
	for _, e := range episodes {
		if e.EpisodeNumber == *candidate.EpisodeNumber && e.Downloaded && e.QualityStatus == model.QualityUpgradeNeeded && e.FilePath != nil && e.DownloadID != nil {
			return &e, nil
		}
	}
	return nil, nil
}

// completeEpisodeUpgrade reuses the season replacement journal and recovery
// protocol, affecting only one verified asset and its owning completion. New
// media is strictly validated by the caller before any playable bytes move.
// Caller holds completionMu. A handled failure retains both copies for replay.
func (s *Service) completeEpisodeUpgrade(ctx context.Context, candidate *model.Download, staging, final string) (bool, bool) {
	fail := func(err error) (bool, bool) {
		s.updateStatus(candidate.ID, model.DownloadStatusPaused, map[string]interface{}{"last_error": fmt.Sprintf("单集升级暂不能切换，保留原版与候选：%v", err)})
		return true, false
	}
	old, err := s.episodeUpgradeState(ctx, candidate)
	if err != nil {
		return fail(err)
	}
	if old == nil {
		return false, false
	}
	root := s.currentMediaRoot(ctx)
	if staging == final || staging == *old.FilePath || !pathWithin(final, root) {
		return fail(fmt.Errorf("升级路径无效"))
	}
	for _, path := range []string{staging, *old.FilePath} {
		if err := checkRetainedPath(path, root); err != nil {
			return fail(err)
		}
	}
	pending, err := filepath.Glob(filepath.Join(root, ".anidog-upgrades", "anime-*", fmt.Sprintf("task-%d", candidate.ID), "*", "journal.json"))
	if err != nil {
		return fail(err)
	}
	if len(pending) != 0 {
		return fail(fmt.Errorf("已有未决切换日志，等待恢复"))
	}
	var owner model.Download
	err = s.db.WithContext(ctx).Scopes(AvailableMedia).Where("anime_id = ? AND episode_number = ? AND (torrent_id = ? OR info_hash = ?) AND file_path = ?", old.AnimeID, old.EpisodeNumber, *old.DownloadID, *old.DownloadID, *old.FilePath).First(&owner).Error
	if err != nil {
		return fail(fmt.Errorf("无法核实原版归属: %w", err))
	}
	attempt := filepath.Join(root, ".anidog-upgrades", fmt.Sprintf("anime-%d", old.AnimeID), fmt.Sprintf("task-%d", candidate.ID), fmt.Sprint(time.Now().UnixNano()))
	backup := filepath.Join(attempt, "previous-episode")
	oldBackup := filepath.Join(backup, filepath.Base(*old.FilePath))
	journal := upgradeJournal{DownloadID: candidate.ID, AnimeID: old.AnimeID, State: "prepared", BackupPath: backup, PreviousEpisodes: []model.AnimeEpisode{*old}, PreviousDownloads: []model.Download{owner}, Moves: []upgradeMove{{*old.FilePath, oldBackup}}}
	for _, pair := range [][2]string{{*old.FilePath, oldBackup}, {staging, final}} {
		moves, err := seasonPackSidecarMoves(pair[0], pair[1])
		if err != nil {
			return fail(err)
		}
		for _, m := range moves {
			journal.Moves = append(journal.Moves, upgradeMove{m.from, m.to})
		}
	}
	journal.Moves = append(journal.Moves, upgradeMove{staging, final})
	size, now := fileSize(staging), time.Now()
	won := s.commitUpgradeJournal(ctx, candidate, filepath.Join(attempt, "journal.json"), &journal, func(tx *gorm.DB) error {
		// Compare the actual owner and quality snapshot again under database locks.
		var current model.AnimeEpisode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, old.ID).Error; err != nil {
			return err
		}
		if current.DownloadID == nil || *current.DownloadID != *old.DownloadID || current.FilePath == nil || *current.FilePath != *old.FilePath || current.QualityStatus != model.QualityUpgradeNeeded {
			return fmt.Errorf("待升级媒体在验收期间已变化")
		}
		result := tx.Model(&model.Download{}).Where("id = ? AND status IN ?", candidate.ID, []string{model.DownloadStatusPending, model.DownloadStatusDownloading}).Updates(map[string]interface{}{"status": model.DownloadStatusCompleted, "progress": 100, "completed_at": now, "file_path": final, "backup_path": backup, "validation_pending": false, "validation_final_path": "", "failure_kind": "", "failure_code": "", "failure_stage": "", "last_error": "", "next_retry_at": nil, "seeking_alternative": false, "quality_note": "单集画质与字幕升级完成；旧版已备份"})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("升级任务状态已变化")
		}
		result = tx.Model(&model.Download{}).Where("id = ? AND status = ? AND file_path = ?", owner.ID, model.DownloadStatusCompleted, *old.FilePath).Updates(map[string]interface{}{"status": model.DownloadStatusSuperseded, "file_path": oldBackup, "backup_path": backup, "quality_note": fmt.Sprintf("已由单集升级任务 #%d 替换，旧版已备份", candidate.ID)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("原媒体归属已变化")
		}
		return tx.Model(&model.AnimeEpisode{}).Where("id = ?", old.ID).Updates(map[string]interface{}{"downloaded": true, "download_id": candidate.TorrentID, "file_path": final, "file_size": size, "quality_status": model.QualityMeetsTarget, "quality_reason": "", "quality_fingerprint": "", "quality_checked_at": now}).Error
	})
	return true, won
}
