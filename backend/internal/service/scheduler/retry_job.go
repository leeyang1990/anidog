// Package scheduler schedules recovery searches independently of download attempts.
package scheduler

import (
	"context"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

// RetryConductor re-evaluates an anime using current preferences. A search is
// not a download attempt: RetryCount belongs to newly created download tasks.
type RetryConductor func(context.Context, *model.Anime)

const recoverySearchInterval = 30 * time.Minute

type RetryFailedJob struct {
	db          *gorm.DB
	conductor   RetryConductor
	maxPerRound int
	mu          sync.Mutex
}

func NewRetryFailedJob(db *gorm.DB, conductor RetryConductor) *RetryFailedJob {
	return &RetryFailedJob{db: db, conductor: conductor, maxPerRound: 50}
}

func (j *RetryFailedJob) Name() string { return "retry_failed" }

func (j *RetryFailedJob) Run(ctx context.Context) {
	if j.conductor == nil || !j.mu.TryLock() {
		return
	}
	defer j.mu.Unlock()
	if ctx.Err() != nil {
		return
	}

	// Only the latest failure of an episode drives recovery. Older attempts
	// remain available to candidate selection for cooldown and diagnostics.
	// NULL picks up legacy rows stranded by the former dispatch logic.
	var rows []model.Download
	err := j.db.WithContext(ctx).
		Where("status = ? AND failure_kind = ? AND retry_count < ?", model.DownloadStatusFailed, model.FailureKindTransient, 3).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", time.Now()).
		Where("anime_id IS NOT NULL AND episode_number IS NOT NULL").
		Where(`NOT EXISTS (SELECT 1 FROM download newer WHERE newer.anime_id = download.anime_id
    AND newer.episode_number = download.episode_number AND newer.status = ? AND newer.id > download.id)`, model.DownloadStatusFailed).
		Find(&rows).Error
	if err != nil {
		zap.L().Warn("retry_failed: 查询失败", zap.Error(err))
		return
	}
	animeIDs, rowIDs := selectRetryBatch(rows, j.maxPerRound)
	selected := make(map[uint]bool, len(rowIDs))
	for _, id := range rowIDs {
		selected[id] = true
	}
	for _, aid := range animeIDs {
		if ctx.Err() != nil {
			return
		}
		var ids []uint
		for _, row := range rows {
			if selected[row.ID] && *row.AnimeID == aid {
				ids = append(ids, row.ID)
			}
		}
		j.recheck(ctx, aid, ids)
	}
}

func (j *RetryFailedJob) recheck(ctx context.Context, animeID uint, rowIDs []uint) {
	// Rearm even if no source exists, storage is unavailable, or the context is
	// canceled while checking. UpdateColumn preserves the actual failure time,
	// which candidate cooldown uses. Unprocessed batches are left untouched.
	defer func() {
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := j.db.WithContext(persistCtx).Model(&model.Download{}).
			Where("id IN ? AND status = ?", rowIDs, model.DownloadStatusFailed).
			UpdateColumn("next_retry_at", time.Now().Add(recoverySearchInterval)).Error; err != nil {
			zap.L().Warn("retry_failed: 保存下次搜索时间失败", zap.Error(err))
		}
	}()
	var anime model.Anime
	if err := j.db.WithContext(ctx).First(&anime, animeID).Error; err != nil {
		zap.L().Warn("retry_failed: 读取番剧失败", zap.Uint("anime_id", animeID), zap.Error(err))
		return
	}
	if !anime.IsSubscribed || anime.MediaManagementState == "archived" {
		return
	}
	zap.L().Info("retry_failed: 重新搜索缺集来源", zap.Uint("anime_id", animeID))
	j.conductor(ctx, &anime)
}

func selectRetryBatch(rows []model.Download, maxAnime int) ([]uint, []uint) {
	animeIDSet := make(map[uint]struct{}, len(rows))
	for _, r := range rows {
		if r.AnimeID != nil {
			animeIDSet[*r.AnimeID] = struct{}{}
		}
	}
	animeIDs := make([]uint, 0, len(animeIDSet))
	for id := range animeIDSet {
		animeIDs = append(animeIDs, id)
	}
	sort.Slice(animeIDs, func(i, k int) bool { return animeIDs[i] < animeIDs[k] })
	if maxAnime > 0 && len(animeIDs) > maxAnime {
		animeIDs = animeIDs[:maxAnime]
	}
	selected := make(map[uint]struct{}, len(animeIDs))
	for _, id := range animeIDs {
		selected[id] = struct{}{}
	}
	rowIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		if r.AnimeID != nil {
			if _, ok := selected[*r.AnimeID]; ok {
				rowIDs = append(rowIDs, r.ID)
			}
		}
	}
	return animeIDs, rowIDs
}

// AbandonedTorrentTTLJob 清理过期的死种黑名单 —— 给 mikan 上同 hash 的新副本
// 一次复活机会。默认 14 天。
//
// 仅清 Kind='transient' / Kind=” 的；Kind='permanent' 保留（虽然目前没人写）。
type AbandonedTorrentTTLJob struct {
	db  *gorm.DB
	ttl time.Duration
}

func NewAbandonedTorrentTTLJob(db *gorm.DB, ttl time.Duration) *AbandonedTorrentTTLJob {
	if ttl <= 0 {
		ttl = 14 * 24 * time.Hour
	}
	return &AbandonedTorrentTTLJob{db: db, ttl: ttl}
}

func (j *AbandonedTorrentTTLJob) Name() string { return "abandoned_torrent_ttl" }

func (j *AbandonedTorrentTTLJob) Run(ctx context.Context) {
	cutoff := time.Now().Add(-j.ttl)
	res := j.db.WithContext(ctx).
		Where("abandoned_at < ?", cutoff).
		Where("kind = ? OR kind = ? OR kind IS NULL", model.FailureKindTransient, "").
		Delete(&model.AbandonedTorrent{})
	if res.Error != nil {
		zap.L().Warn("abandoned_torrent_ttl: 清理失败", zap.Error(res.Error))
		return
	}
	if res.RowsAffected > 0 {
		zap.L().Info("abandoned_torrent_ttl: 已清理过期黑名单",
			zap.Int64("rows", res.RowsAffected),
			zap.Duration("ttl", j.ttl))
	}
}
