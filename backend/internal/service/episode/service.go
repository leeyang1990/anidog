package episode

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/anidog/anidog-go/internal/model"
	bgmsvc "github.com/anidog/anidog-go/internal/service"
)

// Service 维护 anime → animeepisode 的剧集元数据。
//
// 核心职责：从 Bangumi /v0/episodes 同步每集的播出日期 + 标题，
// 让 Orchestrator 知道"这集播没播"，让前端知道"待发布的集啥时候上"。
//
// 同步策略：upsert by (anime_id, episode_number)；
// 已下载状态(downloaded/file_path/file_size)由 Sync 操作 *不* 覆盖，
// 那部分由 download 完成回调维护。
type Service struct {
	db  *gorm.DB
	bgm *bgmsvc.BangumiService
}

func NewService(db *gorm.DB, bgm *bgmsvc.BangumiService) *Service {
	return &Service{db: db, bgm: bgm}
}

// Sync 从 Bangumi 拉取该 anime 的全集表并 upsert 到 animeepisode。
// anime 必须有 BangumiID；否则跳过（返回 nil 不报错）。
func (s *Service) Sync(ctx context.Context, anime *model.Anime) error {
	if anime == nil || anime.BangumiID == nil || *anime.BangumiID == 0 {
		return nil
	}
	eps, err := s.bgm.GetEpisodes(ctx, *anime.BangumiID)
	if err != nil {
		return fmt.Errorf("从 Bangumi 拉取剧集表: %w", err)
	}
	if len(eps) == 0 {
		return nil
	}

	now := time.Now()
	for _, e := range eps {
		// 优先用 ep（正片集号）；缺则用 sort 取整
		epNum := e.Ep
		if epNum <= 0 {
			epNum = int(e.Sort)
		}
		if epNum <= 0 {
			continue
		}

		row := model.AnimeEpisode{
			AnimeID:       anime.ID,
			EpisodeNumber: epNum,
			UpdatedAt:     now,
		}
		// 标题：优先 name_cn，回落 name
		if e.NameCN != "" {
			t := e.NameCN
			row.NameCN = &t
		}
		if e.Name != "" {
			t := e.Name
			row.Title = &t
		}
		if e.AirDate != "" {
			ad := e.AirDate
			row.AirDate = &ad
		}

		// upsert：冲突时只更新元数据，不动 downloaded/file_path/file_size 这些"用户数据"
		err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "anime_id"}, {Name: "episode_number"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"title", "name_cn", "air_date", "updated_at",
			}),
		}).Create(&row).Error
		if err != nil {
			zap.L().Warn("同步 animeepisode 失败",
				zap.Uint("anime_id", anime.ID),
				zap.Int("ep", epNum),
				zap.Error(err))
		}
	}

	zap.L().Info("已同步 animeepisode",
		zap.Uint("anime_id", anime.ID),
		zap.String("title", anime.Title),
		zap.Int("count", len(eps)))
	return nil
}

// SyncAllSubscribed 给所有订阅番剧批量同步集表。供 scheduler.Job 调用。
func (s *Service) SyncAllSubscribed(ctx context.Context) {
	var animes []model.Anime
	if err := s.db.WithContext(ctx).
		Where("is_subscribed = ? AND bangumi_id IS NOT NULL AND bangumi_id > 0", true).
		Find(&animes).Error; err != nil {
		zap.L().Error("查询订阅番剧失败", zap.Error(err))
		return
	}
	for i := range animes {
		// 单个失败不阻塞其他
		if err := s.Sync(ctx, &animes[i]); err != nil {
			zap.L().Warn("同步剧集表失败",
				zap.Uint("anime_id", animes[i].ID),
				zap.Error(err))
		}
	}
	zap.L().Info("剧集表批量同步完成", zap.Int("count", len(animes)))
}

// Run 实现 scheduler.Job 接口
func (s *Service) Run(ctx context.Context) {
	s.SyncAllSubscribed(ctx)
}

// Name 实现 scheduler.Job 接口
func (s *Service) Name() string { return "episode_sync" }

// ListByAnime 返回某 anime 的所有 episode（按集号升序）。
func (s *Service) ListByAnime(ctx context.Context, animeID uint) ([]model.AnimeEpisode, error) {
	var rows []model.AnimeEpisode
	err := s.db.WithContext(ctx).
		Where("anime_id = ?", animeID).
		Order("episode_number ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	// 排序保险（GORM 的 ORDER BY 已生效，这里防御）
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].EpisodeNumber < rows[j].EpisodeNumber
	})
	return rows, nil
}

// IsAired 判断某集是否已播出。规则：
//   - air_date 为空 → 视为已播（缺数据时不阻塞下载，否则可能永远抓不到）
//   - air_date 在今天或之前（按本地时区）→ 已播
//   - air_date 在未来 → 未播
//
// air_date 格式 "YYYY-MM-DD"，按当地零点解析。容错：解析失败 → 已播。
func IsAired(airDate string, now time.Time) bool {
	if airDate == "" {
		return true
	}
	t, err := time.ParseInLocation("2006-01-02", airDate, now.Location())
	if err != nil {
		return true
	}
	// air_date 当天的 23:59 都算已播（避免时区导致"今天才播但服务器算明天"）
	endOfAirDay := t.Add(24*time.Hour - time.Second)
	return !now.Before(t) && !now.After(endOfAirDay) || now.After(t)
}

// ErrNoBangumiID 触发同步时 anime 没有 bangumi_id
var ErrNoBangumiID = errors.New("番剧缺少 bangumi_id，无法同步集表")
