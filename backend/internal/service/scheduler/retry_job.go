// Package scheduler · 重试调度 Job
//
// RetryFailedJob 每 5 分钟扫一次 download 表，把符合条件的 failed 记录"复活"
// 重新交给 Orchestrator 跑一遍 CheckAnime，让多源补救机制再有一次机会。
//
// 触发条件（全部满足）：
//   1. status = 'failed'
//   2. failure_kind = 'transient' （permanent 不重试）
//   3. retry_count < 3
//   4. next_retry_at IS NOT NULL AND next_retry_at <= now（到点）
//   5. 有关联 anime_id + episode_number（手动下载未绑番的不重试）
//
// 重试动作（按 anime_id 聚合，多集失败只触发一次 orchestrator）：
//   a. 把所有到点行 retry_count++、next_retry_at=NULL —— 防止下一轮 5min 后重复触发
//   b. 调用 RetryConductor.CheckAnime(anime) —— 让 orchestrator 对所有缺失集
//      重新挑源。CheckAnime 自己已经避开了正在 downloading/completed/pending
//      的集，所以幂等。失败行的 status 仍是 'failed'，isDuplicate 因为
//      failure_kind='transient' 不会把它当作"6h 冷却中"，于是新一轮可以正常排
//
// 这个 Job 与 orchestrator 主轮询（30min）是叠加关系：
//   - 主轮询：扫所有番剧、关心新的一集播出
//   - RetryFailedJob：只盯失败行、节奏更密（5min），专门救火
package scheduler

import (
	"context"
	"sort"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

// RetryConductor 是 RetryFailedJob 依赖的最小化 orchestrator 接口。
// 实际由 *orchestrator.Orchestrator 实现（main.go 装配时直接传）。
// pref 用 interface{} 是为了避免 scheduler → orchestrator 的 import 反向依赖；
// 调用 CheckAnime 的 orchestrator 实现内部会做类型断言。
type RetryConductor interface {
	CheckAnime(ctx context.Context, anime *model.Anime, prefAny interface{})
}

// GlobalPrefLoader 在每轮 Run 之前拉一次"全局下载偏好"快照。
// 同样用 interface{} 解耦 ——main.go 直接 wrap orchestrator.LoadGlobal 即可。
type GlobalPrefLoader func(ctx context.Context) interface{}

// RetryFailedJob 周期性扫 failed/transient 行并触发对应 anime 的重排。
type RetryFailedJob struct {
	db          *gorm.DB
	conductor   RetryConductor
	loadPref    GlobalPrefLoader
	maxPerRound int // 单次最多处理多少个 anime（防雪崩）
}

func NewRetryFailedJob(db *gorm.DB, conductor RetryConductor, loadPref GlobalPrefLoader) *RetryFailedJob {
	return &RetryFailedJob{
		db:          db,
		conductor:   conductor,
		loadPref:    loadPref,
		maxPerRound: 50,
	}
}

func (j *RetryFailedJob) Name() string { return "retry_failed" }

func (j *RetryFailedJob) Run(ctx context.Context) {
	if j.conductor == nil {
		return
	}
	now := time.Now()

	// 第一步：拉所有"到点"的 transient failed 行
	var rows []model.Download
	if err := j.db.WithContext(ctx).
		Where("status = ?", model.DownloadStatusFailed).
		Where("failure_kind = ?", model.FailureKindTransient).
		Where("retry_count < ?", 3).
		Where("next_retry_at IS NOT NULL").
		Where("next_retry_at <= ?", now).
		Where("anime_id IS NOT NULL").
		Where("episode_number IS NOT NULL").
		Find(&rows).Error; err != nil {
		zap.L().Warn("retry_failed: 查询失败行失败", zap.Error(err))
		return
	}
	if len(rows) == 0 {
		return
	}

	// 第二步：按 anime_id 去重 + 限流 —— 同一个 anime 多集失败只重排一次
	animeIDSet := make(map[uint]struct{}, len(rows))
	rowIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		if r.AnimeID != nil {
			animeIDSet[*r.AnimeID] = struct{}{}
		}
		rowIDs = append(rowIDs, r.ID)
	}
	animeIDs := make([]uint, 0, len(animeIDSet))
	for id := range animeIDSet {
		animeIDs = append(animeIDs, id)
	}
	sort.Slice(animeIDs, func(i, k int) bool { return animeIDs[i] < animeIDs[k] })
	if len(animeIDs) > j.maxPerRound {
		zap.L().Warn("retry_failed: 单轮触发上限，截断",
			zap.Int("total", len(animeIDs)),
			zap.Int("limit", j.maxPerRound))
		animeIDs = animeIDs[:j.maxPerRound]
	}

	// 第三步：更新所有行的 retry_count++ 并清掉 next_retry_at
	//   清空 next_retry_at 是关键 —— 下一轮 RetryFailedJob 不会再选它，
	//   除非新一次失败发生（execute() 失败时会重新计算下次重试时间）
	if err := j.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("id IN ?", rowIDs).
		Updates(map[string]interface{}{
			"retry_count":   gorm.Expr("retry_count + 1"),
			"next_retry_at": nil,
		}).Error; err != nil {
		zap.L().Warn("retry_failed: 标记 retry 失败", zap.Error(err))
		// 不 return —— 即便标记失败，让 orchestrator 跑一次也无害
	}

	zap.L().Info("retry_failed: 开始重新调度",
		zap.Int("failed_rows", len(rows)),
		zap.Int("anime_to_retry", len(animeIDs)))

	// 第四步：拿全局偏好 + 依次跑 CheckAnime
	var pref interface{}
	if j.loadPref != nil {
		pref = j.loadPref(ctx)
	}

	for _, aid := range animeIDs {
		var anime model.Anime
		if err := j.db.WithContext(ctx).First(&anime, aid).Error; err != nil {
			zap.L().Warn("retry_failed: 读 anime 失败", zap.Uint("anime_id", aid), zap.Error(err))
			continue
		}
		if !anime.IsSubscribed {
			continue
		}
		j.conductor.CheckAnime(ctx, &anime, pref)
	}
}

// AbandonedTorrentTTLJob 清理过期的死种黑名单 —— 给 mikan 上同 hash 的新副本
// 一次复活机会。默认 14 天。
//
// 仅清 Kind='transient' / Kind='' 的；Kind='permanent' 保留（虽然目前没人写）。
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
