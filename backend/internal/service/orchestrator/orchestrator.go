package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/episode"
	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/service/setting"
	"github.com/anidog/anidog-go/internal/service/stream"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

// Source constants for Download.Source field
const (
	SourceStream = "stream"
	SourceBT     = "bt"
	SourceRSS    = "rss"
)

// Orchestrator 剧集驱动的多源下载调度器。
type Orchestrator struct {
	db         *gorm.DB
	dlSvc      *dlservice.Service
	streamMgr  *stream.StreamManager
	settingSvc *setting.Service
	indexers   map[string]indexer.Indexer // Name() -> instance
	mikanRSS   *indexer.MikanRSSFetcher   // Mikan RSS（按 mikan_bangumi_id 推送，召回率远超关键词搜索）
	mediaRoot  string                     // BT 下载根目录（容器内路径）
}

// New 构造 Orchestrator。
// indexers 由调用方注册，方便测试替换。传 nil 时自动注册 4 家默认 indexer。
// mediaRoot 为 BT 下载根目录（容器内可写路径），Orchestrator 会按 `<root>/<title> (<year>)/Season <NN>` 组织文件
func New(
	db *gorm.DB,
	dlSvc *dlservice.Service,
	streamMgr *stream.StreamManager,
	settingSvc *setting.Service,
	indexers map[string]indexer.Indexer,
	mediaRoot string,
) *Orchestrator {
	if indexers == nil {
		indexers = map[string]indexer.Indexer{
			"mikan":      indexer.NewMikanIndexer(),
			"dmhy":       indexer.NewDmhyIndexer(),
			"bangumimoe": indexer.NewBangumiMoeIndexer(),
			"nyaa":       indexer.NewNyaaIndexer(),
		}
	}
	if mediaRoot == "" {
		mediaRoot = "/downloads"
	}
	return &Orchestrator{
		db:         db,
		dlSvc:      dlSvc,
		streamMgr:  streamMgr,
		settingSvc: settingSvc,
		indexers:   indexers,
		mikanRSS:   indexer.NewMikanRSSFetcher(),
		mediaRoot:  mediaRoot,
	}
}

// collectBTCandidates 汇总 BT 候选。
//
// 策略（关键：Mikan RSS 召回率远超关键词搜索，这是 ep1/ep4 等小众集数被漏掉的根因）：
//  1. 若 anime.MikanBangumiID 已反查并写入 → 走 /RSS/Bangumi?bangumiId=X
//     一次性拉到该番所有字幕组的所有集（典型 30-300 条），InfoHash 直接在 link 末段。
//  2. 同时也跑关键词 Aggregate（Dmhy / BangumiMoe / Nyaa 等其他源仍要参与）。
//  3. 按 InfoHash 去重，Mikan RSS 的条目优先。
//  4. 给所有条目补 Parsed 字段。
//
// 失败处理：Mikan RSS 拿不到不阻塞，回退到关键词搜索。
func (o *Orchestrator) collectBTCandidates(ctx context.Context, anime *model.Anime, enabled []indexer.Indexer) []indexer.Candidate {
	var rssCands []indexer.Candidate
	if anime.MikanBangumiID != nil && *anime.MikanBangumiID > 0 && o.mikanRSS != nil {
		rssCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		items, err := o.mikanRSS.Fetch(rssCtx, *anime.MikanBangumiID)
		cancel()
		if err != nil {
			zap.L().Warn("Mikan RSS 拉取失败，回退关键词搜索",
				zap.String("anime", anime.Title),
				zap.Int("mikan_bangumi_id", *anime.MikanBangumiID),
				zap.Error(err))
		} else {
			for i := range items {
				items[i].Parsed = titleparse.Parse(items[i].Title)
			}
			rssCands = items
			zap.L().Info("Mikan RSS 命中",
				zap.String("anime", anime.Title),
				zap.Int("mikan_bangumi_id", *anime.MikanBangumiID),
				zap.Int("count", len(items)))
		}
	}

	// 关键词搜索路径（其他 indexer 仍要参与）
	kwCands := indexer.Aggregate(ctx, enabled, anime.Title)

	// 合并去重（InfoHash 优先；空则用 Title）
	seen := make(map[string]bool, len(rssCands)+len(kwCands))
	merged := make([]indexer.Candidate, 0, len(rssCands)+len(kwCands))
	for _, c := range rssCands {
		key := c.InfoHash
		if key == "" {
			key = c.Title
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, c)
	}
	for _, c := range kwCands {
		key := c.InfoHash
		if key == "" {
			key = c.Title
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, c)
	}
	return merged
}

// Run 实现 scheduler.Job 接口：定时对所有订阅番剧逐个跑 CheckAnime。
func (o *Orchestrator) Run(ctx context.Context) {
	o.CheckAllSubscribed(ctx)
}

// CheckAllSubscribed 遍历所有订阅番剧，逐个检查并填坑。
// 同时满足 scheduler.BangumiChecker 接口，可直接替换旧的 bangumi.AutoDownloader。
func (o *Orchestrator) CheckAllSubscribed(ctx context.Context) {
	global := LoadGlobal(ctx, o.settingSvc)

	var animes []model.Anime
	if err := o.db.WithContext(ctx).Where("is_subscribed = ?", true).Find(&animes).Error; err != nil {
		zap.L().Error("orchestrator: 查询订阅番剧失败", zap.Error(err))
		return
	}

	zap.L().Info("orchestrator: 开始扫描", zap.Int("anime_count", len(animes)))

	for i := range animes {
		a := &animes[i]
		o.CheckAnime(ctx, a, global)
	}

	zap.L().Info("orchestrator: 扫描完成")
}

// CheckAnime 检查单个番剧，对缺失集按优先级尝试各源下载。
// global 传入全局偏好；函数内部合并 per-anime override。
func (o *Orchestrator) CheckAnime(ctx context.Context, anime *model.Anime, global Preference) {
	if anime == nil || !anime.IsSubscribed {
		return
	}

	pref := MergeWithAnime(global, anime)

	// 期望集数
	expected := 0
	if anime.EpisodeCount != nil {
		expected = *anime.EpisodeCount
	}
	if expected <= 0 {
		zap.L().Debug("orchestrator: 番剧无集数信息，跳过", zap.String("title", anime.Title))
		return
	}

	// 已下载的集（跨所有 source_type）
	downloaded := o.downloadedEpisodes(ctx, anime.ID)

	// 各集播出时间（来自 animeepisode.air_date，由 episode.Service 同步自 Bangumi）
	airDates := o.episodeAirDates(ctx, anime.ID)
	now := time.Now()

	missing := make([]int, 0, expected)
	skippedUnaired := make([]int, 0)
	for ep := 1; ep <= expected; ep++ {
		if downloaded[ep] {
			continue
		}
		if ad, has := airDates[ep]; has && !episode.IsAired(ad, now) {
			// 还没播出 —— 不要去搜，省掉无意义的请求和"未命中"诊断
			skippedUnaired = append(skippedUnaired, ep)
			continue
		}
		missing = append(missing, ep)
	}
	if len(missing) == 0 {
		if len(skippedUnaired) > 0 {
			zap.L().Debug("orchestrator: 番剧无可下载集（剩余均为待发布）",
				zap.String("title", anime.Title),
				zap.Ints("upcoming", skippedUnaired))
		}
		return
	}

	zap.L().Info("orchestrator: 检查番剧",
		zap.String("title", anime.Title),
		zap.Uint("anime_id", anime.ID),
		zap.Ints("missing_episodes", missing),
		zap.Ints("upcoming_episodes", skippedUnaired),
	)

	for _, ep := range missing {
		success := o.tryDownloadEpisode(ctx, anime, ep, pref)
		if !success {
			zap.L().Debug("orchestrator: 本轮未下载",
				zap.String("title", anime.Title),
				zap.Int("episode", ep))
		}
	}
}

// tryDownloadEpisode 按优先级尝试每个源下载指定集，成功（入队）即返回 true。
func (o *Orchestrator) tryDownloadEpisode(ctx context.Context, anime *model.Anime, ep int, pref Preference) bool {
	retryCount := o.retryCountForEpisode(ctx, anime.ID, ep)
	for _, srcType := range pref.Priority {
		if pref.IsSourceDisabled(srcType) {
			o.recordDiag(anime.ID, ep, srcType, 0, 0, "源已禁用", "", 0)
			continue
		}

		var (
			diagResultCount int
			diagRankedOut   int
			diagReason      string
			diagBestTitle   string
			diagBestScore   float64
			ok              bool
		)

		switch srcType {
		case SourceStream:
			ok, diagResultCount, diagReason = o.tryStream(ctx, anime, ep, retryCount)
		case SourceBT:
			ok, diagResultCount, diagRankedOut, diagReason, diagBestTitle, diagBestScore = o.tryBT(ctx, anime, ep, pref, retryCount)
		default:
			// SourceRSS 已不再作为 orchestrator 的主动源（被动通道由 RSSRefreshJob 处理）。
			// 其他未知 srcType 也忽略。
			continue
		}

		o.recordDiag(anime.ID, ep, srcType, diagResultCount, diagRankedOut, diagReason, diagBestTitle, diagBestScore)
		if ok {
			return true
		}
	}
	return false
}

// ---- Stream 源适配 ----

func (o *Orchestrator) tryStream(ctx context.Context, anime *model.Anime, ep, retryCount int) (ok bool, resultCount int, reason string) {
	if o.streamMgr == nil {
		return false, 0, "stream manager 未就绪"
	}
	// 去重 / 失败冷却（避免每 30min 重复创建同一个失败的 stream 任务）
	if o.isDuplicate(ctx, anime.ID, ep, SourceStream) {
		return false, 0, "该集已有 stream 记录或在失败冷却期内，跳过"
	}

	rule, detailURL, episodes, switched, err := o.resolveStreamSource(ctx, anime, ep)
	if err != nil {
		return false, 0, err.Error()
	}

	preferredRoad := ""
	if anime.StreamRoadName != nil {
		preferredRoad = *anime.StreamRoadName
	}
	if switched {
		preferredRoad = ""
	}
	failedRoads := o.failedStreamRoads(ctx, anime.ID, ep)
	epInfo, roadName, found := selectStreamEpisode(episodes, ep, preferredRoad, failedRoads, rule.MultiSources)
	if !found {
		return false, len(episodes), fmt.Sprintf("所有播放线路均无第 %d 集", ep)
	}

	// 入队
	epCopy := ep
	task := &dlservice.Task{
		Name:            fmt.Sprintf("%s - 第%02d集", anime.Title, ep),
		URL:             epInfo.URL,
		DownloadType:    model.DownloadTypeStream,
		Source:          SourceStream,
		AnimeName:       anime.Title,
		AnimeID:         &anime.ID,
		EpisodeNumber:   &epCopy,
		StreamRuleID:    &rule.ID,
		StreamRule:      rule,
		StreamDetailURL: detailURL,
		StreamRoadName:  roadName,
		RetryCount:      retryCount,
	}
	if _, err := o.dlSvc.Create(ctx, task); err != nil {
		return false, len(episodes), "创建下载任务失败: " + err.Error()
	}
	return true, len(episodes), fmt.Sprintf("已从线路 %s 入队: %s", roadName, epInfo.Name)
}

// resolveStreamSource 优先使用当前健康源；当前规则 broken、页面解析失败或缺少目标集时，
// 自动在其他启用且未熔断的规则中重新搜索，并将成功候选持久化为番剧的新源。
func (o *Orchestrator) resolveStreamSource(ctx context.Context, anime *model.Anime, ep int) (*model.StreamRule, string, []stream.EpisodeInfo, bool, error) {
	if anime.StreamRuleID != nil && anime.StreamDetailURL != nil && *anime.StreamDetailURL != "" {
		var current model.StreamRule
		if err := o.db.WithContext(ctx).First(&current, *anime.StreamRuleID).Error; err == nil &&
			!isRuleBroken(&current) && !o.hasStreamRuleFailure(ctx, anime.ID, ep, current.ID) {
			episodes, parseErr := o.streamMgr.GetEpisodes(ctx, &current, *anime.StreamDetailURL)
			if parseErr == nil && hasEpisode(episodes, ep) {
				return &current, *anime.StreamDetailURL, episodes, false, nil
			}
		}
	}

	var rules []model.StreamRule
	q := o.db.WithContext(ctx).Where("enabled = ?", true)
	if anime.StreamRuleID != nil {
		q = q.Where("id <> ?", *anime.StreamRuleID)
	}
	if err := q.Find(&rules).Error; err != nil {
		return nil, "", nil, false, fmt.Errorf("查询备用 stream 规则失败: %w", err)
	}
	sort.SliceStable(rules, func(i, j int) bool { return streamRuleHealthScore(&rules[i]) > streamRuleHealthScore(&rules[j]) })
	for i := range rules {
		rule := &rules[i]
		if isRuleBroken(rule) {
			continue
		}
		searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		results, searchErr := o.streamMgr.SearchAnime(searchCtx, rule, anime.Title)
		cancel()
		if searchErr != nil || len(results) == 0 {
			continue
		}
		best := stream.PickBestMatch(anime.Title, results)
		if best == nil {
			continue
		}
		parseCtx, parseCancel := context.WithTimeout(ctx, 30*time.Second)
		episodes, parseErr := o.streamMgr.GetEpisodes(parseCtx, rule, best.URL)
		parseCancel()
		if parseErr != nil || !hasEpisode(episodes, ep) {
			continue
		}
		road := firstRoadWithEpisode(episodes, ep)
		updates := map[string]interface{}{
			"stream_rule_id": rule.ID, "stream_rule_name": rule.Name,
			"stream_detail_url": best.URL, "stream_road_name": road,
			"source_health_status": "", "source_health_note": "", "source_health_at": nil,
		}
		if err := o.db.WithContext(ctx).Model(anime).Updates(updates).Error; err != nil {
			continue
		}
		anime.StreamRuleID, anime.StreamRuleName = &rule.ID, &rule.Name
		anime.StreamDetailURL, anime.StreamRoadName = &best.URL, &road
		zap.L().Info("orchestrator: 自动切换 stream 规则", zap.String("anime", anime.Title), zap.String("rule", rule.Name), zap.String("candidate", best.Name))
		return rule, best.URL, episodes, true, nil
	}
	return nil, "", nil, false, fmt.Errorf("当前 stream 规则不可用，所有健康备用规则均未找到第 %d 集", ep)
}

// hasStreamRuleFailure 防止同一集在规则整体仍 healthy 时反复撞同一个坏播放页。
// 只要该集在该规则有失败记录，本轮就换其他规则；源恢复由长周期 half-open 负责。
func (o *Orchestrator) hasStreamRuleFailure(ctx context.Context, animeID uint, ep int, ruleID uint) bool {
	var count int64
	o.db.WithContext(ctx).Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND stream_rule_id = ? AND download_type = ? AND status = ?",
			animeID, ep, ruleID, model.DownloadTypeStream, model.DownloadStatusFailed).
		Count(&count)
	return count > 0
}

func isRuleBroken(rule *model.StreamRule) bool {
	if rule.HealthStatus == nil || *rule.HealthStatus != "broken" {
		return false
	}
	// broken 规则冷却 1 小时后进入 half-open，由一次真实任务探测是否恢复。
	return rule.HealthAt == nil || time.Since(*rule.HealthAt) < time.Hour
}

func streamRuleHealthScore(rule *model.StreamRule) int {
	if rule.HealthStatus == nil || *rule.HealthStatus == "" {
		return 2
	}
	switch *rule.HealthStatus {
	case "healthy":
		return 3
	case "degraded":
		return 1
	default:
		return 0
	}
}

// ---- BT 源适配 ----

func (o *Orchestrator) tryBT(ctx context.Context, anime *model.Anime, ep int, pref Preference, retryCount int) (
	ok bool, resultCount int, rankedOut int, reason, bestTitle string, bestScore float64,
) {
	if o.dlSvc == nil || !o.dlSvc.HasExecutor(model.DownloadTypeTorrent) {
		return false, 0, 0, "qBittorrent 当前不可用，跳过 BT 入队", "", 0
	}
	// 选启用的 indexer
	enabled := make([]indexer.Indexer, 0, len(pref.EnabledIndexers))
	for _, name := range pref.EnabledIndexers {
		if ix, has := o.indexers[name]; has {
			enabled = append(enabled, ix)
		}
	}
	if len(enabled) == 0 {
		return false, 0, 0, "无启用的 BT indexer", "", 0
	}

	cands := o.collectBTCandidates(ctx, anime, enabled)
	resultCount = len(cands)
	if resultCount == 0 {
		return false, 0, 0, "所有 indexer 均无结果", "", 0
	}

	ranked := indexer.RankByPreference(cands, pref.ToIndexerPref(), ep)
	rankedOut = resultCount - len(ranked)
	if len(ranked) == 0 {
		return false, resultCount, rankedOut, "所有候选均不符合偏好（集数不匹配/分辨率不符/字幕组不符）", "", 0
	}

	// 否决黑名单以及本番已经失败过的 InfoHash。必须在选 top 之前过滤，
	// 否则第一名失败后会每轮都再次选中第一名，并在后面的去重检查处停止，
	// 永远轮不到第二候选。
	rankedFiltered := ranked[:0]
	for _, c := range ranked {
		if c.InfoHash != "" && (o.hasHistoricalFailure(ctx, c.InfoHash) ||
			o.hasAnimeInfoHashFailure(ctx, anime.ID, c.InfoHash)) {
			rankedOut++
			continue
		}
		rankedFiltered = append(rankedFiltered, c)
	}
	ranked = rankedFiltered
	if len(ranked) == 0 {
		return false, resultCount, rankedOut, "所有候选 InfoHash 均有历史失败记录，跳过", "", 0
	}

	// 死种探活：对每个候选并发 scrape UDP tracker，3s 超时。
	// 行为：scrape 成功且 seeders=0 → 剔除；scrape 失败（超时/UDP 不通） → 保留（fail-open）。
	// 这样在没法 scrape 的网络环境下不会把所有候选都误杀。
	scrapeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	type scrapedCand struct {
		c       indexer.ScoredCandidate
		ok      bool // scrape 收到回包
		seeders int
	}
	scrapedCh := make(chan scrapedCand, len(ranked))
	var swg sync.WaitGroup
	for i := range ranked {
		c := ranked[i]
		magnet := c.MagnetURL
		if magnet == "" {
			magnet = c.InfoHash
		}
		if magnet == "" {
			scrapedCh <- scrapedCand{c: c, ok: false} // 缺 hash 不能 scrape，保留
			continue
		}
		swg.Add(1)
		go func() {
			defer swg.Done()
			r := indexer.ScrapeMagnet(scrapeCtx, magnet)
			scrapedCh <- scrapedCand{c: c, ok: r.OK, seeders: r.Seeders}
		}()
	}
	go func() { swg.Wait(); close(scrapedCh); cancel() }()

	alive := make([]indexer.ScoredCandidate, 0, len(ranked))
	scrapedDead := 0
	scrapedNoResponse := 0
	for sc := range scrapedCh {
		switch {
		case sc.ok && sc.seeders > 0:
			// 探活成功且有活种
			sc.c.Seeders = sc.seeders
			sc.c.SeedersReported = true
			alive = append(alive, sc.c)
		case sc.ok && sc.seeders == 0:
			// 探活成功但无活种 → 死种
			scrapedDead++
		default:
			// scrape 没回包：保留（fail-open）
			scrapedNoResponse++
			alive = append(alive, sc.c)
		}
	}
	rankedOut += scrapedDead
	if len(alive) == 0 {
		return false, resultCount, rankedOut,
			fmt.Sprintf("所有候选经 UDP scrape 探活均无活种（剔除 %d 个）", scrapedDead),
			"", 0
	}
	if scrapedDead > 0 || scrapedNoResponse > 0 {
		zap.L().Info("BT 候选 scrape 结果",
			zap.Int("alive", len(alive)),
			zap.Int("dead", scrapedDead),
			zap.Int("no_response", scrapedNoResponse))
	}
	// 全员无回包 → UDP egress 大概率被代理/防火墙拦了，scrape 功能实际禁用
	// （典型现象：开发机 Clash 在 macOS Docker 下劫持 DNS 不代理 UDP）
	if scrapedNoResponse == len(ranked)+scrapedDead && scrapedNoResponse > 0 {
		zap.L().Warn("所有候选 UDP scrape 均无回包，疑似 UDP 出站被拦截，本轮 scrape 实际禁用（fail-open）",
			zap.Int("candidates", scrapedNoResponse))
	}
	ranked = alive

	top := ranked[0]
	bestTitle = top.Title
	bestScore = top.Score

	// 入队用的 URL
	url := top.MagnetURL
	if url == "" {
		url = top.TorrentURL
	}
	if url == "" {
		return false, resultCount, rankedOut, "候选缺少 magnet/torrent URL", bestTitle, bestScore
	}

	// 去重 1：该 (anime, ep, source_type) 是否已有 downloading/completed 的记录
	if o.isDuplicate(ctx, anime.ID, ep, SourceBT) {
		return false, resultCount, rankedOut, "已有 BT 下载记录（同集），跳过", bestTitle, bestScore
	}
	// 去重 2：同一个 anime 下同一 URL 已经提交过（批量包场景：01-12 Fin 不要为每集重复入队）
	if o.isDuplicateURL(ctx, anime.ID, url) {
		return true, resultCount, rankedOut, "合集种子已入队（当前集在批量包内）", bestTitle, bestScore
	}
	// 去重 3：同 anime 下同 InfoHash 已经存在记录（含 failed/completed），不论 source/episode。
	// 防止 sync 误标 failed 后 Orchestrator 再次拿同 magnet 入队产生双胞胎。
	if o.isDuplicateInfoHash(ctx, anime.ID, top.InfoHash) {
		return false, resultCount, rankedOut, "该 InfoHash 已存在历史记录，跳过", bestTitle, bestScore
	}

	// 入队（使用 magnet，走 torrent 执行器）
	epCopy := ep

	task := &dlservice.Task{
		Name:          fmt.Sprintf("%s - 第%02d集", anime.Title, ep),
		URL:           url,
		DownloadType:  model.DownloadTypeTorrent,
		Source:        SourceBT,
		AnimeName:     anime.Title,
		AnimeID:       &anime.ID,
		EpisodeNumber: &epCopy,
		SavePath:      dlservice.BuildAnimeSavePath(o.mediaRoot, anime),
		RetryCount:    retryCount,
	}
	if _, err := o.dlSvc.Create(ctx, task); err != nil {
		return false, resultCount, rankedOut, "创建下载任务失败: " + err.Error(), bestTitle, bestScore
	}
	zap.L().Info("orchestrator: BT 下载入队",
		zap.String("anime", anime.Title),
		zap.Int("ep", ep),
		zap.String("source", top.SourceName),
		zap.String("title", top.Title),
		zap.Float64("score", top.Score),
	)
	return true, resultCount, rankedOut, fmt.Sprintf("已从 %s 入队（score=%.1f）", top.SourceName, top.Score), bestTitle, bestScore
}

// ---- RSS 源说明 ----
// RSS 不再是 orchestrator 的主动源。被动通道由 RSSRefreshJob + rssrule 处理：
// 定时刷新 feeds → 解析入 rssentry 表 → 命中规则即下载。
// 若需要诊断 RSS 命中情况，请查 rssentry 表，不要在 tryDownloadEpisode 里再开一档。

// ---- 辅助 ----

// retryCountForEpisode 返回同一番剧/集数失败链已经消耗的最大重试次数。
// RetryFailedJob 会先递增到期旧记录，再调用 Orchestrator；新任务继承这个值，
// 因而即使发生 BT/Stream 换源，也不会重新从第 0 次开始计算退避。
func (o *Orchestrator) retryCountForEpisode(ctx context.Context, animeID uint, ep int) int {
	var retryCount int
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND status = ?", animeID, ep, model.DownloadStatusFailed).
		Select("COALESCE(MAX(retry_count), 0)").
		Scan(&retryCount)
	return retryCount
}

// failedStreamRoads 返回该集已经失败过的线路。换源时优先选择从未失败的线路，
// 所有线路都失败过时再回到首选线路，交给全局退避上限收敛。
func (o *Orchestrator) failedStreamRoads(ctx context.Context, animeID uint, ep int) map[string]bool {
	var roads []string
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND download_type = ? AND status = ?",
			animeID, ep, model.DownloadTypeStream, model.DownloadStatusFailed).
		Where("stream_road_name IS NOT NULL AND stream_road_name <> ''").
		Distinct("stream_road_name").
		Pluck("stream_road_name", &roads)
	out := make(map[string]bool, len(roads))
	for _, road := range roads {
		out[road] = true
	}
	return out
}

// selectStreamEpisode 按线路组织扁平剧集列表，并选择指定集。
// 顺序：未失败的首选线路 → 未失败的其他线路 → 已失败线路。
func selectStreamEpisode(episodes []stream.EpisodeInfo, ep int, preferred string, failed map[string]bool, multi bool) (stream.EpisodeInfo, string, bool) {
	if ep <= 0 {
		return stream.EpisodeInfo{}, "", false
	}
	byRoad := make(map[string][]stream.EpisodeInfo)
	order := make([]string, 0)
	for _, item := range episodes {
		road := item.RoadName
		if _, exists := byRoad[road]; !exists {
			order = append(order, road)
		}
		byRoad[road] = append(byRoad[road], item)
	}
	if preferred != "" {
		for i, road := range order {
			if road == preferred {
				reordered := []string{road}
				reordered = append(reordered, order[:i]...)
				reordered = append(reordered, order[i+1:]...)
				order = reordered
				break
			}
		}
	}
	if !multi && len(order) > 1 {
		order = order[:1]
	}
	candidates := append([]string{}, order...)
	for _, allowFailed := range []bool{false, true} {
		for _, road := range candidates {
			if failed[road] != allowFailed {
				continue
			}
			if item, ok := findEpisode(byRoad[road], ep); ok {
				return item, road, true
			}
		}
	}
	return stream.EpisodeInfo{}, "", false
}

var (
	episodeNameRe = regexp.MustCompile(`(?i)(?:^|[^0-9])(?:第\s*|EP(?:ISODE)?\s*)?(\d{1,4})(?:\.\d+)?(?:\s*[集话話])?(?:$|[^0-9])`)
	episodeURLRe  = regexp.MustCompile(`[-_/](\d{1,4})(?:\.html?)?(?:[?#].*)?$`)
)

func parseEpisodeNumber(item stream.EpisodeInfo) (int, bool) {
	for _, candidate := range []struct {
		re *regexp.Regexp
		s  string
	}{{episodeNameRe, strings.TrimSpace(item.Name)}, {episodeURLRe, item.URL}} {
		match := candidate.re.FindStringSubmatch(candidate.s)
		if len(match) < 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

func findEpisode(items []stream.EpisodeInfo, ep int) (stream.EpisodeInfo, bool) {
	hasExplicit := false
	for _, item := range items {
		if n, ok := parseEpisodeNumber(item); ok {
			hasExplicit = true
			if n == ep {
				return item, true
			}
		}
	}
	// 只有整条线路完全无法解析集数时才允许按位置兜底，防止倒序列表错集。
	if !hasExplicit && ep > 0 && ep <= len(items) {
		return items[ep-1], true
	}
	return stream.EpisodeInfo{}, false
}

func hasEpisode(episodes []stream.EpisodeInfo, ep int) bool {
	_, _, ok := selectStreamEpisode(episodes, ep, "", nil, true)
	return ok
}

func firstRoadWithEpisode(episodes []stream.EpisodeInfo, ep int) string {
	_, road, _ := selectStreamEpisode(episodes, ep, "", nil, true)
	return road
}

// downloadedEpisodes 查询某 anime 的已完成/进行中集数（跨所有 source_type）
func (o *Orchestrator) downloadedEpisodes(ctx context.Context, animeID uint) map[int]bool {
	var rows []struct {
		EpisodeNumber *int
		Status        string
	}
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number IS NOT NULL", animeID).
		Where("status IN ?", []string{
			model.DownloadStatusCompleted,
			model.DownloadStatusDownloading,
			model.DownloadStatusPending,
		}).
		Select("episode_number, status").
		Scan(&rows)

	out := make(map[int]bool, len(rows))
	for _, r := range rows {
		if r.EpisodeNumber != nil {
			out[*r.EpisodeNumber] = true
		}
	}
	return out
}

// episodeAirDates 查询某 anime 各集的播出日期（YYYY-MM-DD），来自 animeepisode 表。
// 由 episode.Service 定时从 Bangumi /v0/episodes 同步。
func (o *Orchestrator) episodeAirDates(ctx context.Context, animeID uint) map[int]string {
	var rows []model.AnimeEpisode
	o.db.WithContext(ctx).
		Where("anime_id = ? AND air_date IS NOT NULL AND air_date <> ''", animeID).
		Find(&rows)
	out := make(map[int]string, len(rows))
	for _, r := range rows {
		if r.AirDate != nil && *r.AirDate != "" {
			out[r.EpisodeNumber] = *r.AirDate
		}
	}
	return out
}

// isDuplicate 判断 (anime_id, ep, source_type) 是否应该跳过本轮入队。
// 跳过条件：
//  1. 已存在 downloading/completed/pending 记录（同集同源）
//  2. 累计 failed 次数 ≥ 3 —— 该源对该集多次失败，永久跳过
//  3. 最近 6 小时内有 failed 记录 —— 短期冷却
//
// transient 例外（见 download.classifyError）：
//
//	只有 permanent 失败才计入条件 2/3。failure_kind='transient' 的失败由
//	RetryFailedJob 按退避节奏负责重投，orchestrator 不参与节流 —— 否则
//	transient 失败也被 6h 冷却锁住会跟自动重试打架。
func (o *Orchestrator) isDuplicate(ctx context.Context, animeID uint, ep int, sourceType string) bool {
	sources := []string{sourceType}
	if sourceType == SourceStream {
		// 历史 AutoDownloader 用 bangumi 表示触发来源；本质仍是 stream。
		sources = append(sources, dlservice.SourceBangumi)
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source IN ?", animeID, ep, sources).
		Where("status IN ?", []string{
			model.DownloadStatusDownloading,
			model.DownloadStatusCompleted,
			model.DownloadStatusPending,
		}).
		Count(&count)
	if count > 0 {
		return true
	}
	// 尚未到退避时间的 transient 失败必须阻止常规 30min 扫描提前重投。
	var coolingTransient int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source IN ?", animeID, ep, sources).
		Where("status = ? AND failure_kind = ?", model.DownloadStatusFailed, model.FailureKindTransient).
		Where("next_retry_at IS NOT NULL AND next_retry_at > ?", time.Now()).
		Count(&coolingTransient)
	if coolingTransient > 0 {
		return true
	}
	// 累计 permanent 失败 ≥ 3 次：永久放弃该源
	var totalPermFailed int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source IN ?", animeID, ep, sources).
		Where("status = ?", model.DownloadStatusFailed).
		Where("failure_kind = ? OR failure_kind = ''", model.FailureKindPermanent).
		Count(&totalPermFailed)
	if totalPermFailed >= 3 {
		return true
	}
	// permanent 配置/IO 错冷却 6 小时；快速预算耗尽的外部源错误 1 小时后
	// half-open 探测。这样无需人工介入，也不会持续轰击故障站点。
	var recentPermFailed int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source IN ?", animeID, ep, sources).
		Where("status = ?", model.DownloadStatusFailed).
		Where("((failure_kind = ? OR failure_kind = '') AND updated_at > ?) OR (failure_kind = ? AND updated_at > ?)",
			model.FailureKindPermanent, time.Now().Add(-6*time.Hour),
			model.FailureKindExhausted, time.Now().Add(-time.Hour)).
		Count(&recentPermFailed)
	return recentPermFailed > 0
}

// isDuplicateInfoHash 判断同一 anime + InfoHash 是否已存在记录（任何状态）。
// 用于 BT 入队前的硬保护：避免 sync 误判把任务标 failed 后，下一轮 Orchestrator
// 拿同一个 magnet 又开一行（典型现象：1268 与 1286 同 hash 双胞胎）。
func (o *Orchestrator) isDuplicateInfoHash(ctx context.Context, animeID uint, infoHash string) bool {
	if infoHash == "" {
		return false
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND info_hash = ?", animeID, strings.ToUpper(infoHash)).
		Count(&count)
	return count > 0
}

// hasHistoricalFailure 判断某个 InfoHash 是否在黑名单（abandoned_torrent）里。
// 用于在排序后剔除"已知没活种"的 magnet：例如 qbit_sync 把超过 6h 仍 0 元数据
// 的种子写入黑名单后，Orchestrator 下一轮看到同 hash 的候选就直接跳过。
//
// 注意：之前这里查的是 download.status='failed'，会被各种瞬时失败误伤
// （比如 qBit 短暂连不上、用户手动删除等）；现在仅查"已确诊死种"的黑名单。
func (o *Orchestrator) hasHistoricalFailure(ctx context.Context, infoHash string) bool {
	if infoHash == "" {
		return false
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.AbandonedTorrent{}).
		Where("info_hash = ?", strings.ToUpper(infoHash)).
		Count(&count)
	return count > 0
}

// hasAnimeInfoHashFailure 判断该番是否已经尝试并失败过某个种子。
// 与全局 abandoned_torrent 不同，它只影响当前番，避免解析误关联污染其他番剧。
func (o *Orchestrator) hasAnimeInfoHashFailure(ctx context.Context, animeID uint, infoHash string) bool {
	if infoHash == "" {
		return false
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND info_hash = ? AND status = ?",
			animeID, strings.ToUpper(infoHash), model.DownloadStatusFailed).
		Count(&count)
	return count > 0
}

// 用于批量包（Fin 合集）场景：同一个 magnet 不应为每一集都重复入队。
func (o *Orchestrator) isDuplicateURL(ctx context.Context, animeID uint, url string) bool {
	if url == "" {
		return false
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND url = ?", animeID, url).
		Where("status IN ?", []string{
			model.DownloadStatusDownloading,
			model.DownloadStatusCompleted,
			model.DownloadStatusPending,
			model.DownloadStatusFailed,
		}).
		Count(&count)
	return count > 0
}

// recordDiag 记录一次源尝试的诊断信息（失败和成功都记）
func (o *Orchestrator) recordDiag(animeID uint, ep int, srcType string, resultCount, rankedOut int, reason, bestTitle string, bestScore float64) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return
	}
	rec := model.OrchestratorDiagnosis{
		AnimeID:       animeID,
		EpisodeNumber: ep,
		SourceType:    srcType,
		CheckedAt:     time.Now(),
		ResultCount:   resultCount,
		RankedOut:     rankedOut,
		Reason:        reason,
		BestTitle:     bestTitle,
		BestScore:     bestScore,
	}
	// 覆盖写入：先删同 (anime, ep, src) 的历史，保证 UI 只看最新
	o.db.Where("anime_id = ? AND episode_number = ? AND source_type = ?", animeID, ep, srcType).
		Delete(&model.OrchestratorDiagnosis{})
	if err := o.db.Create(&rec).Error; err != nil {
		zap.L().Warn("orchestrator: 写诊断记录失败", zap.Error(err))
	}
}
