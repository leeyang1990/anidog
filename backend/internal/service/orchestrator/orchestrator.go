package orchestrator

import (
	"context"
	"fmt"
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
		mediaRoot:  mediaRoot,
	}
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
			ok, diagResultCount, diagReason = o.tryStream(ctx, anime, ep)
		case SourceBT:
			ok, diagResultCount, diagRankedOut, diagReason, diagBestTitle, diagBestScore = o.tryBT(ctx, anime, ep, pref)
		case SourceRSS:
			ok, diagResultCount, diagReason = o.tryRSS(ctx, anime, ep)
		default:
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

func (o *Orchestrator) tryStream(ctx context.Context, anime *model.Anime, ep int) (ok bool, resultCount int, reason string) {
	if anime.StreamRuleID == nil || anime.StreamDetailURL == nil {
		return false, 0, "该番剧未配置流媒体源"
	}
	if o.streamMgr == nil {
		return false, 0, "stream manager 未就绪"
	}
	// 去重 / 失败冷却（避免每 30min 重复创建同一个失败的 stream 任务）
	if o.isDuplicate(ctx, anime.ID, ep, SourceStream) {
		return false, 0, "该集已有 stream 记录或在失败冷却期内，跳过"
	}

	var rule model.StreamRule
	if err := o.db.WithContext(ctx).First(&rule, *anime.StreamRuleID).Error; err != nil {
		return false, 0, fmt.Sprintf("找不到规则 id=%d", *anime.StreamRuleID)
	}
	// 规则被健康检查标 broken/degraded 跳过 —— 不要再用一个已知坏/烂的源浪费请求
	if rule.HealthStatus != nil {
		switch *rule.HealthStatus {
		case "broken":
			return false, 0, "stream 规则状态 broken，跳过"
		case "degraded":
			return false, 0, "stream 规则状态 degraded，跳过"
		}
	}

	episodes, err := o.streamMgr.GetEpisodes(ctx, &rule, *anime.StreamDetailURL)
	if err != nil || len(episodes) == 0 {
		return false, len(episodes), fmt.Sprintf("获取剧集失败: %v", err)
	}

	roadName := ""
	if anime.StreamRoadName != nil {
		roadName = *anime.StreamRoadName
	}
	var filtered []stream.EpisodeInfo
	if roadName != "" {
		for _, e := range episodes {
			if e.RoadName == roadName {
				filtered = append(filtered, e)
			}
		}
	}
	if len(filtered) == 0 {
		filtered = episodes
	}

	// 取第 ep 集（索引从 0 开始）
	if ep-1 >= len(filtered) {
		return false, len(filtered), fmt.Sprintf("清单只有 %d 集，无第 %d 集", len(filtered), ep)
	}
	epInfo := filtered[ep-1]

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
		StreamRule:      &rule,
		StreamDetailURL: *anime.StreamDetailURL,
		StreamRoadName:  roadName,
	}
	if _, err := o.dlSvc.Create(ctx, task); err != nil {
		return false, len(filtered), "创建下载任务失败: " + err.Error()
	}
	return true, len(filtered), fmt.Sprintf("已入队: %s", epInfo.Name)
}

// ---- BT 源适配 ----

func (o *Orchestrator) tryBT(ctx context.Context, anime *model.Anime, ep int, pref Preference) (
	ok bool, resultCount int, rankedOut int, reason, bestTitle string, bestScore float64,
) {
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

	cands := indexer.Aggregate(ctx, enabled, anime.Title)
	resultCount = len(cands)
	if resultCount == 0 {
		return false, 0, 0, "所有 indexer 均无结果", "", 0
	}

	ranked := indexer.RankByPreference(cands, pref.ToIndexerPref(), ep)
	rankedOut = resultCount - len(ranked)
	if len(ranked) == 0 {
		return false, resultCount, rankedOut, "所有候选均不符合偏好（集数不匹配/分辨率不符/字幕组不符）", "", 0
	}

	// 否决曾经失败过的 InfoHash —— 防止把已知没活种的 magnet 反复入队
	// （Mikan 等不上报 seeders 的源在 RankByPreference 那里无法过滤死种，得在这兜底）
	rankedFiltered := ranked[:0]
	for _, c := range ranked {
		if c.InfoHash != "" && o.hasHistoricalFailure(ctx, c.InfoHash) {
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

// ---- RSS 源适配 ----
// 查询 rss_entry 表中已关联到该 anime 的 entry，如果存在匹配当前集的未下载记录，
// 表示 RSS 已抓到该集但尚未触发下载（比如被规则过滤）—— 目前 RSS engine 已自动入队，
// 所以这里主要用于展示诊断；如需强制下载，未来可扩展 "从该 entry 创建下载"。
func (o *Orchestrator) tryRSS(ctx context.Context, anime *model.Anime, ep int) (ok bool, resultCount int, reason string) {
	var entries []model.RSSEntry
	o.db.WithContext(ctx).
		Where("matched_anime_id = ?", anime.ID).
		Find(&entries)

	matching := 0
	for _, e := range entries {
		if e.ParsedEpisode != nil && *e.ParsedEpisode == ep {
			matching++
			if !e.Downloaded {
				// 该集的 RSS entry 存在但没下载成功（可能被规则过滤），跳过让其他源接手
				return false, matching, "RSS entry 存在但未下载（可能被规则过滤）"
			}
		}
	}

	if matching == 0 {
		return false, 0, "无匹配的 RSS entry（等待 feed 更新）"
	}
	return false, matching, "RSS entry 已下载"
}

// ---- 辅助 ----

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
//   1. 已存在 downloading/completed/pending 记录（同集同源）
//   2. 累计 failed 次数 ≥ 3 —— 该源对该集多次失败，永久跳过（避免无意义重试）
//   3. 最近 6 小时内有 failed 记录 —— 短期冷却
func (o *Orchestrator) isDuplicate(ctx context.Context, animeID uint, ep int, sourceType string) bool {
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source = ?", animeID, ep, sourceType).
		Where("status IN ?", []string{
			model.DownloadStatusDownloading,
			model.DownloadStatusCompleted,
			model.DownloadStatusPending,
		}).
		Count(&count)
	if count > 0 {
		return true
	}
	// 累计失败 ≥ 3 次：永久放弃该源
	var totalFailed int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source = ?", animeID, ep, sourceType).
		Where("status = ?", model.DownloadStatusFailed).
		Count(&totalFailed)
	if totalFailed >= 3 {
		return true
	}
	// 最近 6 小时 failed 冷却（指数退避近似）
	var recentFailed int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source = ?", animeID, ep, sourceType).
		Where("status = ?", model.DownloadStatusFailed).
		Where("created_at > ?", time.Now().Add(-6*time.Hour)).
		Count(&recentFailed)
	return recentFailed > 0
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
