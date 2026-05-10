package bangumi

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/stream"
)

// healthScore 规则健康状态权重（越大越优先选择）
func healthScore(status *string) int {
	if status == nil {
		return 2 // 未检测：中等优先级
	}
	switch *status {
	case "healthy":
		return 3
	case "", "unknown":
		return 2
	case "degraded":
		return 1
	case "broken":
		return 0
	}
	return 2
}

type AutoDownloader struct {
	db        *gorm.DB
	dlSvc     dlservice.Downloader
	streamMgr *stream.StreamManager
}

func NewAutoDownloader(db *gorm.DB, dlSvc dlservice.Downloader, streamMgr *stream.StreamManager) *AutoDownloader {
	return &AutoDownloader{db: db, dlSvc: dlSvc, streamMgr: streamMgr}
}

// TriggerManualCheck 手动触发：先清除该 anime 的 failed 记录，再发现下载。
// 用于用户点"检查更新"时避免因历史 failed 堆积而不重新尝试。
func (a *AutoDownloader) TriggerManualCheck(ctx context.Context, animeID uint, title string) {
	if animeID == 0 {
		return
	}
	// 清除 failed 记录，让 queueEpisodes 能重新创建
	res := a.db.WithContext(ctx).
		Where("anime_id = ? AND status = ?", animeID, model.DownloadStatusFailed).
		Delete(&model.Download{})
	if res.RowsAffected > 0 {
		zap.L().Info("清除失败下载以便重试", zap.Uint("anime_id", animeID), zap.Int64("rows", res.RowsAffected))
	}
	a.TriggerAutoDownload(ctx, animeID, title)
}

// TriggerAutoDownload 追番后触发：搜索源 → 保存偏好 → 下载所有集。
// 如已有源偏好则走增量下载。
func (a *AutoDownloader) TriggerAutoDownload(ctx context.Context, animeID uint, title string) {
	if title == "" || a.streamMgr == nil {
		return
	}

	var anime model.Anime
	if err := a.db.WithContext(ctx).First(&anime, animeID).Error; err != nil {
		zap.L().Error("获取番剧信息失败", zap.Uint("anime_id", animeID), zap.Error(err))
		return
	}

	// 已有源偏好：增量下载
	if anime.StreamDetailURL != nil && *anime.StreamDetailURL != "" && anime.StreamRuleID != nil {
		a.downloadMissingEpisodes(ctx, &anime)
		return
	}

	// 无源偏好：自动发现并下载所有集
	a.discoverAndDownloadAll(ctx, &anime)
}

// CheckAllSubscribed 定时任务：扫描所有已追番番剧，下载新集。
func (a *AutoDownloader) CheckAllSubscribed(ctx context.Context) {
	var animes []model.Anime
	if err := a.db.WithContext(ctx).
		Where("is_subscribed = ?", true).
		Find(&animes).Error; err != nil {
		zap.L().Error("查询已追番番剧失败", zap.Error(err))
		return
	}

	zap.L().Info("定时检查追番更新", zap.Int("count", len(animes)))
	for i := range animes {
		anime := &animes[i]
		if anime.StreamDetailURL != nil && *anime.StreamDetailURL != "" && anime.StreamRuleID != nil {
			// 已有源偏好：增量检查
			a.downloadMissingEpisodes(ctx, anime)
		} else {
			// 没源偏好：尝试发现（之前可能搜索失败，定时重试）
			a.discoverAndDownloadAll(ctx, anime)
		}
	}
}

// discoverAndDownloadAll 自动发现源：
//   - 收集所有源的候选（每源最佳一个）并按匹配度排序
//   - 选第一个立即保存偏好 + 批量入队下载
//   - 不做同步验证（异步健康检测由 SourceHealthJob 承担）
func (a *AutoDownloader) discoverAndDownloadAll(ctx context.Context, anime *model.Anime) {
	var rules []model.StreamRule
	if err := a.db.WithContext(ctx).Where("enabled = ?", true).Find(&rules).Error; err != nil || len(rules) == 0 {
		zap.L().Info("没有启用的流媒体规则", zap.Uint("anime_id", anime.ID))
		return
	}

	// 按规则健康度排序：healthy > unknown > degraded > broken
	sort.SliceStable(rules, func(i, j int) bool {
		return healthScore(rules[i].HealthStatus) > healthScore(rules[j].HealthStatus)
	})

	zap.L().Info("开始自动匹配下载源", zap.String("title", anime.Title), zap.Int("规则数", len(rules)))

	for i := range rules {
		rule := &rules[i]
		results, err := a.streamMgr.SearchAnime(ctx, rule, anime.Title)
		if err != nil || len(results) == 0 {
			continue
		}
		best := stream.PickBestMatch(anime.Title, results)
		if best == nil {
			continue
		}
		episodes, err := a.streamMgr.GetEpisodes(ctx, rule, best.URL)
		if err != nil || len(episodes) == 0 {
			continue
		}
		roadName, filtered := pickFirstRoad(episodes)

		// 保存源偏好
		a.savePreference(ctx, anime, rule, best.URL, roadName)

		zap.L().Info("匹配成功",
			zap.String("title", anime.Title),
			zap.String("rule", rule.Name),
			zap.String("candidate", best.Name),
			zap.String("road", roadName),
			zap.Int("episodes", len(filtered)))

		// 批量下载（不阻塞等待）
		a.queueEpisodes(ctx, anime, rule, filtered)
		return
	}

	zap.L().Info("未找到匹配下载源", zap.String("title", anime.Title))
}

// savePreference 保存源偏好到 anime 记录
func (a *AutoDownloader) savePreference(ctx context.Context, anime *model.Anime, rule *model.StreamRule, detailURL, roadName string) {
	updates := map[string]interface{}{
		"stream_rule_id":    rule.ID,
		"stream_detail_url": detailURL,
		"stream_road_name":  roadName,
		"stream_rule_name":  rule.Name,
	}
	if err := a.db.WithContext(ctx).Model(anime).Updates(updates).Error; err != nil {
		zap.L().Error("保存源偏好失败", zap.Error(err))
		return
	}
	anime.StreamRuleID = &rule.ID
	anime.StreamDetailURL = &detailURL
	anime.StreamRoadName = &roadName
	anime.StreamRuleName = &rule.Name
}

// downloadMissingEpisodes 用已保存的源偏好检查并下载缺失集。
func (a *AutoDownloader) downloadMissingEpisodes(ctx context.Context, anime *model.Anime) {
	var rule model.StreamRule
	if err := a.db.WithContext(ctx).First(&rule, *anime.StreamRuleID).Error; err != nil {
		zap.L().Error("获取规则失败", zap.Uint("rule_id", *anime.StreamRuleID), zap.Error(err))
		return
	}

	episodes, err := a.streamMgr.GetEpisodes(ctx, &rule, *anime.StreamDetailURL)
	if err != nil || len(episodes) == 0 {
		zap.L().Debug("获取剧集列表失败或为空", zap.String("title", anime.Title), zap.Error(err))
		return
	}

	// 按清单过滤
	roadName := ""
	if anime.StreamRoadName != nil {
		roadName = *anime.StreamRoadName
	}
	var filtered []stream.EpisodeInfo
	if roadName != "" {
		for _, ep := range episodes {
			if ep.RoadName == roadName {
				filtered = append(filtered, ep)
			}
		}
	}
	if len(filtered) == 0 {
		filtered = episodes
	}

	a.queueEpisodes(ctx, anime, &rule, filtered)
}

// queueEpisodes 批量创建下载任务，跳过已下载/已在队列的集。
func (a *AutoDownloader) queueEpisodes(ctx context.Context, anime *model.Anime, rule *model.StreamRule, episodes []stream.EpisodeInfo) {
	existing := a.getExistingEpisodes(ctx, anime.ID, anime.StreamDetailURL, anime.StreamRoadName)

	roadName := ""
	if anime.StreamRoadName != nil {
		roadName = *anime.StreamRoadName
	}
	detailURL := ""
	if anime.StreamDetailURL != nil {
		detailURL = *anime.StreamDetailURL
	}

	created := 0
	for i, ep := range episodes {
		epNum := i + 1
		if existing[epNum] {
			continue
		}

		epName := fmt.Sprintf("%s - %s", anime.Title, ep.Name)
		epNumCopy := epNum
		task := &dlservice.Task{
			Name:            epName,
			URL:             ep.URL,
			DownloadType:    model.DownloadTypeStream,
			Source:          dlservice.SourceBangumi,
			AnimeName:       anime.Title,
			AnimeID:         &anime.ID,
			StreamRuleID:    &rule.ID,
			StreamDetailURL: detailURL,
			StreamRoadName:  roadName,
			StreamRule:    rule,
			EpisodeNumber: &epNumCopy,
		}

		if _, err := a.dlSvc.Create(ctx, task); err != nil {
			zap.L().Error("创建下载任务失败", zap.String("episode", epName), zap.Error(err))
			continue
		}
		created++
	}

	if created > 0 {
		zap.L().Info("已加入下载队列",
			zap.String("title", anime.Title),
			zap.Int("new", created),
			zap.Int("total", len(episodes)),
		)
	}
}

// getExistingEpisodes 已有下载记录的集（包括 failed，避免定时任务重复创建）。
// getExistingEpisodes 已有下载记录的集（按 detail_url + road 精确过滤）
func (a *AutoDownloader) getExistingEpisodes(ctx context.Context, animeID uint, detailURL, roadName *string) map[int]bool {
	var downloads []model.Download
	q := a.db.WithContext(ctx).Where("anime_id = ?", animeID)
	if detailURL != nil && *detailURL != "" {
		q = q.Where("stream_detail_url = ?", *detailURL)
	}
	if roadName != nil && *roadName != "" {
		q = q.Where("stream_road_name = ?", *roadName)
	}
	q.Find(&downloads)

	set := make(map[int]bool, len(downloads))
	for _, d := range downloads {
		if d.EpisodeNumber != nil {
			set[*d.EpisodeNumber] = true
		}
	}
	return set
}

// pickFirstRoad 从剧集列表中选第一条播放线路。
func pickFirstRoad(episodes []stream.EpisodeInfo) (string, []stream.EpisodeInfo) {
	if len(episodes) == 0 {
		return "", nil
	}
	firstRoad := episodes[0].RoadName
	if firstRoad == "" {
		return "", episodes
	}
	var filtered []stream.EpisodeInfo
	for _, ep := range episodes {
		if ep.RoadName == firstRoad {
			filtered = append(filtered, ep)
		}
	}
	return firstRoad, filtered
}
