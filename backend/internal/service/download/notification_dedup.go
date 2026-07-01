package download

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/anidog/anidog-go/internal/model"
)

// claimEpisodeNotification 是 "<番剧> S<NN>E<MM> 已更新" 通知的幂等闸门。
//
// 调用方在准备 Broadcast 之前先调一次：
//   - 返回 true  → 这一对 (anime_id, episode_number) 是首次 → 继续 Broadcast
//   - 返回 false → 已经发过（或同时被另一路径 claim 了）→ 直接 return，不要再发
//
// 实现：依赖 episode_notification 表上 (anime_id, episode_number) 唯一索引。
// INSERT ... ON CONFLICT DO NOTHING：
//   - 没冲突 → RowsAffected=1 → claim 成功
//   - 冲突   → RowsAffected=0 → 已经有人 claim 过了
//
// animeID == 0 或 episode <= 0 时返回 true（无法去重的场景就放行；这种是
// 手动下载没绑番、或者下载没解析出集数，本来一年也发不了几次）。
//
// 任何 DB 错误都按 "失败放行" 处理（fail-open）—— 通知偶尔重发比通知丢失更可接受。
func claimEpisodeNotification(db *gorm.DB, animeID uint, episode int, season int) bool {
	if animeID == 0 || episode <= 0 {
		return true
	}
	rec := model.EpisodeNotification{
		AnimeID:       animeID,
		EpisodeNumber: episode,
		Season:        season,
		NotifiedAt:    time.Now(),
	}
	res := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rec)
	if res.Error != nil {
		zap.L().Warn("通知去重 claim 失败，放行（可能重发一次）",
			zap.Uint("anime_id", animeID),
			zap.Int("episode", episode),
			zap.Error(res.Error))
		return true
	}
	if res.RowsAffected == 0 {
		zap.L().Info("该集通知已发过，跳过重发",
			zap.Uint("anime_id", animeID),
			zap.Int("episode", episode))
		return false
	}
	return true
}
