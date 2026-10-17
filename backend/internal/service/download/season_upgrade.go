package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/episode"
	"github.com/anidog/anidog-go/internal/service/stream"
)

// WithLibraryLock prevents the disk reconciler from observing a half-swapped
// season. Single-episode finalization and pack finalization use the same lock.
func (s *Service) WithLibraryLock(fn func() error) error {
	s.completionMu.Lock()
	defer s.completionMu.Unlock()
	return fn()
}

// CheckAdditionalCapacity reserves room for the incoming copy without counting
// on deletion of the playable old season. Existing queue payloads also count.
func (s *Service) CheckAdditionalCapacity(ctx context.Context, bytes int64) error {
	s.capacityMu.Lock()
	defer s.capacityMu.Unlock()
	if bytes <= 0 {
		return s.ensureMediaCapacity(ctx)
	}
	return s.ensureTaskCapacity(ctx, &Task{EstimatedBytes: bytes}, 0)
}

type upgradeMove struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type upgradeJournal struct {
	DownloadID        uint                 `json:"download_id"`
	AnimeID           uint                 `json:"anime_id"`
	State             string               `json:"state"`
	BackupPath        string               `json:"backup_path"`
	Moves             []upgradeMove        `json:"moves"`
	PreviousEpisodes  []model.AnimeEpisode `json:"previous_episodes"`
	PreviousDownloads []model.Download     `json:"previous_downloads"`
}

func writeUpgradeJournal(path string, j *upgradeJournal) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(path+".tmp", path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func rollbackUpgrade(j *upgradeJournal) error {
	for i := len(j.Moves) - 1; i >= 0; i-- {
		m := j.Moves[i]
		if _, err := os.Stat(m.From); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if _, err := os.Stat(m.To); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(m.From), 0755); err != nil {
			return err
		}
		if err := os.Rename(m.To, m.From); err != nil {
			return err
		}
	}
	return nil
}

// RecoverSeasonUpgrades runs before the BT engine or scheduler starts. A DB
// commit is the decision point: committed swaps stay, interrupted swaps roll
// back. Journals retain the previous file mappings for manual recovery as well.
func (s *Service) RecoverSeasonUpgrades(ctx context.Context) error {
	root := s.currentMediaRoot(ctx)
	journals, err := filepath.Glob(filepath.Join(root, ".anidog-upgrades", "anime-*", "task-*", "*", "journal.json"))
	if err != nil {
		return err
	}
	for _, path := range journals {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var j upgradeJournal
		if err = json.Unmarshal(data, &j); err != nil {
			return err
		}
		if j.State == "committed" || j.State == "rolled_back" {
			continue
		}
		for _, m := range j.Moves {
			if !pathWithin(m.From, root) || !pathWithin(m.To, root) {
				return fmt.Errorf("升级日志路径越界: %s", path)
			}
		}
		var d model.Download
		if err = s.db.WithContext(ctx).First(&d, j.DownloadID).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == nil && d.Status == model.DownloadStatusCompleted && d.BackupPath == j.BackupPath {
			j.State = "committed"
		} else {
			if err = rollbackUpgrade(&j); err != nil {
				return fmt.Errorf("回滚升级 #%d: %w", j.DownloadID, err)
			}
			j.State = "rolled_back"
		}
		if err = writeUpgradeJournal(path, &j); err != nil {
			return err
		}
	}
	return nil
}

// Caller holds completionMu. All episodes are validated before any old media
// moves. Two directory renames switch the library, with durable recovery for a
// process crash between filesystem work and the database commit.
func (s *Service) completeSeasonUpgrade(ctx context.Context, candidate *model.Download, files map[int]string, mediaRoot string) bool {
	var anime model.Anime
	if err := s.db.WithContext(ctx).First(&anime, *candidate.AnimeID).Error; err != nil {
		return false
	}
	var old []model.AnimeEpisode
	if err := s.db.WithContext(ctx).Where("anime_id = ?", anime.ID).Order("episode_number").Find(&old).Error; err != nil {
		return false
	}
	if !episode.SeasonFinished(&anime, old, time.Now()) {
		_, _ = s.Pause(candidate.ID)
		s.db.Model(candidate).Update("last_error", "尚未确认完结，整季升级已暂停；原版仍可播放")
		return false
	}
	total := *anime.EpisodeCount
	if len(files) != total {
		s.rejectMediaCandidate(ctx, candidate, candidateMediaError{fmt.Errorf("升级合集未完整覆盖全部 %d 集正片", total)})
		return false
	}
	needed := false
	for _, row := range old {
		if row.Downloaded && row.QualityStatus == model.QualityUpgradeNeeded {
			needed = true
		}
	}
	if !needed {
		if s.updateStatus(candidate.ID, model.DownloadStatusSuperseded, map[string]interface{}{"quality_note": "现有版本已无确认的升级需求，保留原文件"}) {
			if e := s.executors[candidate.DownloadType]; e != nil {
				_ = e.Remove(executorTaskID(candidate), false)
			}
		}
		return false
	}
	if candidate.UpgradeTarget != "" {
		quality, languages, _, err := s.mediaGoal(ctx, candidate.AnimeID)
		if err != nil {
			return false
		}
		if QualityGoalKey(quality, languages) != candidate.UpgradeTarget {
			if s.updateStatus(candidate.ID, model.DownloadStatusSuperseded, map[string]interface{}{"quality_note": "下载偏好已改变，保留旧版并重新评估升级"}) {
				if e := s.executors[candidate.DownloadType]; e != nil {
					_ = e.Remove(executorTaskID(candidate), false)
				}
			}
			return false
		}
	}
	if candidate.SavePath == nil || !IsTorrentRaceSavePath(*candidate.SavePath) || !pathWithin(*candidate.SavePath, mediaRoot) {
		s.rejectMediaCandidate(ctx, candidate, candidateMediaError{fmt.Errorf("整季升级必须在当前媒体根目录的隔离路径中验收")})
		return false
	}
	// Strict real-file validation applies even when ordinary preference fallback
	// is enabled. A season replacement must achieve the target, not merely play.
	for ep := 1; ep <= total; ep++ {
		path := files[ep]
		if path == "" || !pathWithin(path, *candidate.SavePath) || isUpgradeExtraPath(path, *candidate.SavePath) {
			s.rejectMediaCandidate(ctx, candidate, candidateMediaError{fmt.Errorf("升级合集缺少明确对应的第 %d 集正片", ep)})
			return false
		}
		if err := s.validateCompletedMedia(ctx, candidate, ep, path); err != nil {
			s.rejectMediaCandidate(ctx, candidate, err)
			return false
		}
		if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
			return false
		}
	}
	seasonDir := BuildAnimeSavePath(mediaRoot, &anime)
	attempt := filepath.Join(mediaRoot, ".anidog-upgrades", fmt.Sprintf("anime-%d", anime.ID), fmt.Sprintf("task-%d", candidate.ID), strconv.FormatInt(time.Now().UnixNano(), 10))
	ready, backup := filepath.Join(attempt, "new-season"), filepath.Join(attempt, "previous-season")
	journalPath := filepath.Join(attempt, "journal.json")
	if err := os.MkdirAll(ready, 0755); err != nil {
		return false
	}
	j := upgradeJournal{DownloadID: candidate.ID, AnimeID: anime.ID, State: "prepared", BackupPath: backup, PreviousEpisodes: old}
	if err := s.db.WithContext(ctx).Where("anime_id = ? AND status = ?", anime.ID, model.DownloadStatusCompleted).Find(&j.PreviousDownloads).Error; err != nil {
		return false
	}
	plans := make([]seasonPackMovePlan, 0, total)
	for ep := 1; ep <= total; ep++ {
		from := files[ep]
		final := stream.BuildMediaPath(mediaRoot, &anime, ep, strings.ToLower(filepath.Ext(from)))
		to := filepath.Join(ready, filepath.Base(final))
		plans = append(plans, seasonPackMovePlan{ep: ep, from: from, to: final, size: fileSize(from)})
		j.Moves = append(j.Moves, upgradeMove{from, to})
		sidecars, err := seasonPackSidecarMoves(from, to)
		if err != nil {
			return false
		}
		for _, m := range sidecars {
			j.Moves = append(j.Moves, upgradeMove{m.from, m.to})
		}
	}
	if info, err := os.Lstat(seasonDir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		// Preserve season metadata and Extras in the active library. All old videos
		// and their original sidecars remain together in the recoverable backup.
		entries, err := os.ReadDir(seasonDir)
		if err != nil {
			return false
		}
		for _, entry := range entries {
			name := entry.Name()
			from, to := filepath.Join(seasonDir, name), filepath.Join(ready, name)
			if entry.IsDir() && strings.EqualFold(name, "Extras") {
				j.Moves = append(j.Moves, upgradeMove{from, to})
				continue
			}
			ext := strings.ToLower(filepath.Ext(name))
			if entry.IsDir() || (ext != ".nfo" && ext != ".jpg" && ext != ".png" && ext != ".webp") {
				continue
			}
			data, e := os.ReadFile(from)
			if e != nil {
				return false
			}
			if e = os.WriteFile(to, data, 0644); e != nil {
				return false
			}
		}
		j.Moves = append(j.Moves, upgradeMove{seasonDir, backup})
	} else if !os.IsNotExist(err) {
		return false
	}
	j.Moves = append(j.Moves, upgradeMove{ready, seasonDir})
	if err := writeUpgradeJournal(journalPath, &j); err != nil {
		return false
	}
	fail := func(err error) bool {
		rollbackErr := rollbackUpgrade(&j)
		if rollbackErr == nil {
			j.State = "rolled_back"
			_ = writeUpgradeJournal(journalPath, &j)
		}
		note := fmt.Sprintf("整季升级切换失败，原版已保留: %v", err)
		if rollbackErr != nil {
			note = fmt.Sprintf("整季升级需恢复，已暂停: %v；回滚: %v", err, rollbackErr)
			s.updateStatus(candidate.ID, model.DownloadStatusPaused, map[string]interface{}{"last_error": note})
		} else {
			s.db.Model(candidate).Update("last_error", note)
		}
		zap.L().Error(note)
		return false
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
	now := time.Now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := tx.Model(&model.Download{}).Where("id = ? AND status IN ?", candidate.ID, []string{model.DownloadStatusPending, model.DownloadStatusDownloading}).Updates(map[string]interface{}{"status": model.DownloadStatusCompleted, "progress": 100, "completed_at": now, "file_path": seasonDir, "backup_path": backup, "failure_kind": "", "last_error": "", "next_retry_at": nil, "seeking_alternative": false, "quality_note": fmt.Sprintf("整季 %d 集升级完成；旧版已备份", total)})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return fmt.Errorf("升级任务状态已变化")
		}
		for _, d := range j.PreviousDownloads {
			changes := map[string]interface{}{"status": model.DownloadStatusSuperseded, "quality_note": fmt.Sprintf("已由整季升级任务 #%d 替换，旧版已备份", candidate.ID), "backup_path": backup}
			if d.FilePath != nil && pathWithin(*d.FilePath, seasonDir) {
				rel, _ := filepath.Rel(seasonDir, *d.FilePath)
				changes["file_path"] = filepath.Join(backup, rel)
			}
			if err := tx.Model(&model.Download{}).Where("id = ? AND status = ?", d.ID, model.DownloadStatusCompleted).Updates(changes).Error; err != nil {
				return err
			}
		}
		for _, p := range plans {
			id, path, size := candidate.TorrentID, p.to, p.size
			row := model.AnimeEpisode{AnimeID: anime.ID, EpisodeNumber: p.ep, Downloaded: true, DownloadID: &id, FilePath: &path, FileSize: &size, QualityStatus: model.QualityMeetsTarget, QualityCheckedAt: &now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "anime_id"}, {Name: "episode_number"}}, DoUpdates: clause.Assignments(map[string]interface{}{"downloaded": true, "download_id": id, "file_path": path, "file_size": size, "quality_status": model.QualityMeetsTarget, "quality_reason": "", "quality_fingerprint": "", "quality_checked_at": now})}).Create(&row).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.Anime{}).Where("id = ?", anime.ID).Updates(map[string]interface{}{"media_management_state": "archived", "current_episode": total}).Error
	})
	if err != nil {
		return fail(err)
	}
	j.State = "committed"
	if err = writeUpgradeJournal(journalPath, &j); err != nil {
		zap.L().Warn("整季升级已提交，启动时将补齐日志", zap.Error(err))
	}
	eps := make([]int, total)
	for i := range eps {
		eps[i] = i + 1
	}
	s.settleSeasonPack(ctx, candidate, eps)
	if e := s.executors[candidate.DownloadType]; e != nil {
		_ = e.Remove(executorTaskID(candidate), false)
	}
	s.notifyCompletion(candidate.ID)
	return true
}

func (s *Service) validateSeasonUpgradeTask(ctx context.Context, task *Task) error {
	if task.Purpose != model.DownloadPurposeSeasonUpgrade {
		return nil
	}
	if task.AnimeID == nil || task.Scope != model.DownloadScopeSeason {
		return fmt.Errorf("整季升级必须绑定番剧和完整合集")
	}
	var a model.Anime
	var eps []model.AnimeEpisode
	if err := s.db.WithContext(ctx).First(&a, *task.AnimeID).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Where("anime_id = ?", a.ID).Find(&eps).Error; err != nil {
		return err
	}
	if !episode.SeasonFinished(&a, eps, time.Now()) {
		return fmt.Errorf("尚未确认完结，不能整季升级")
	}
	if task.EpisodeStart == nil || task.EpisodeEnd == nil || *task.EpisodeStart != 1 || *task.EpisodeEnd != *a.EpisodeCount {
		return fmt.Errorf("升级合集必须覆盖全部正片")
	}
	needed := false
	for _, ep := range eps {
		if ep.Downloaded && ep.QualityStatus == model.QualityUpgradeNeeded {
			needed = true
		}
	}
	if !needed {
		return fmt.Errorf("没有确认的质量升级需求")
	}
	return nil
}

// A crash after the DB commit may precede engine cleanup. Never let that stale
// torrent fetch another copy into the old staging directory after restart.
func (s *Service) SettleRecoveredSeasonUpgrades(ctx context.Context) {
	var completed []model.Download
	if err := s.db.WithContext(ctx).Where("purpose = ? AND status = ? AND backup_path <> ''", model.DownloadPurposeSeasonUpgrade, model.DownloadStatusCompleted).Find(&completed).Error; err != nil {
		return
	}
	for i := range completed {
		d := &completed[i]
		if e := s.executors[d.DownloadType]; e != nil {
			if err := e.Remove(executorTaskID(d), false); err != nil {
				zap.L().Warn("清理已完成升级的下载任务失败，文件保持不变", zap.Uint("id", d.ID), zap.Error(err))
			}
		}
	}
}
