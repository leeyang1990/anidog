package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/episode"
	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/service/setting"
	"github.com/anidog/anidog-go/internal/service/stream"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

// Source constants for Download.Source field
const (
	SourceStream = "stream"
	SourceBT     = "bt"
	SourceMikan  = "mikan"
	SourceRSS    = "rss"
)

// Orchestrator 剧集驱动的多源下载调度器。
type Orchestrator struct {
	db             *gorm.DB
	dlSvc          *dlservice.Service
	streamMgr      streamCatalog
	settingSvc     *setting.Service
	indexers       map[string]indexer.Indexer // Name() -> instance
	mikanRSS       *indexer.MikanRSSFetcher   // Mikan RSS（按 mikan_bangumi_id 推送，召回率远超关键词搜索）
	mediaRoot      string                     // BT 下载根目录（容器内路径）
	indexerHTTP    *http.Client               // 复用动态代理的 BT/Mikan 客户端
	animeLocks     sync.Map                   // anime id -> *sync.Mutex（单实例内防并发审计/入队）
	mediaProbe     func(context.Context, string) (mediacheck.Media, error)
	qualityAudited sync.Map // anime id -> bool（每次进程启动做一次历史流媒体时长复核）
	candidateMu    sync.Mutex
	candidates     map[string]candidateCacheEntry // 同一轮缺多集时复用 BT 元数据，避免请求量按集数放大
}

type candidateCacheEntry struct {
	result    indexer.SearchResult
	expiresAt time.Time
}

const candidateCacheTTL = 20 * time.Minute

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
	clients ...*http.Client,
) *Orchestrator {
	var indexerHTTP *http.Client
	if len(clients) > 0 {
		indexerHTTP = clients[0]
	}
	if indexers == nil {
		indexers = indexer.NewDefaultIndexers(indexerHTTP)
	}
	if mediaRoot == "" {
		mediaRoot = "/downloads"
	}
	o := &Orchestrator{
		db:          db,
		dlSvc:       dlSvc,
		settingSvc:  settingSvc,
		indexers:    indexers,
		mikanRSS:    indexer.NewMikanRSSFetcher(),
		mediaRoot:   mediaRoot,
		indexerHTTP: indexerHTTP,
		candidates:  make(map[string]candidateCacheEntry),
	}
	// Avoid a typed nil inside the catalog interface when streaming is disabled.
	if streamMgr != nil {
		o.streamMgr = streamMgr
	}
	if indexerHTTP != nil {
		o.mikanRSS.Client = indexerHTTP
	}
	return o
}

// collectBTCandidates 汇总一个 BT 层级的候选。includeMikan 为 false 时只跑
// 普通 BT indexer；为 true 时才绑定 Mikan 番剧并合并 Mikan RSS。
//
// 策略（关键：Mikan RSS 召回率远超关键词搜索，这是 ep1/ep4 等小众集数被漏掉的根因）：
//  1. 若 anime.MikanBangumiID 已反查并写入 → 走 /RSS/Bangumi?bangumiId=X
//     一次性拉到该番所有字幕组的所有集（典型 30-300 条），InfoHash 直接在 link 末段。
//  2. 同时也跑关键词 Aggregate（Dmhy / BangumiMoe / Nyaa 等其他源仍要参与）。
//  3. 按 InfoHash 去重，Mikan RSS 的条目优先。
//  4. 给所有条目补 Parsed 字段。
//
// 失败处理：Mikan RSS 拿不到不阻塞，回退到关键词搜索。
func (o *Orchestrator) collectBTCandidates(
	ctx context.Context,
	anime *model.Anime,
	enabled []indexer.Indexer,
	includeMikan bool,
) indexer.SearchResult {
	// 旧数据或订阅时直连失败可能没有 mikan_bangumi_id。每次真正缺集时用
	// 动态代理客户端按多个标题补查一次，命中后持久化，后续直接走高召回 RSS。
	if includeMikan && (anime.MikanBangumiID == nil || *anime.MikanBangumiID <= 0) && o.indexerHTTP != nil {
		season := 0
		if anime.Season != nil {
			season = *anime.Season
		}
		keywords := []string{anime.Title}
		for _, title := range []*string{anime.OfficialTitle, anime.OriginalTitle} {
			if title != nil && strings.TrimSpace(*title) != "" && *title != anime.Title {
				keywords = append(keywords, *title)
			}
		}
		for _, keyword := range keywords {
			lookupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			mid, matchedTitle, err := indexer.LookupMikanBangumiID(lookupCtx, keyword, season, o.indexerHTTP)
			cancel()
			if err != nil || mid <= 0 || !titleparse.InstallmentCompatible(anime.Title, matchedTitle) {
				continue
			}
			anime.MikanBangumiID = &mid
			o.db.WithContext(ctx).Model(anime).Update("mikan_bangumi_id", mid)
			zap.L().Info("orchestrator: 自动绑定 Mikan 番剧", zap.String("anime", anime.Title), zap.Int("mikan_bangumi_id", mid), zap.String("matched_title", matchedTitle))
			break
		}
	}

	cacheKey := o.candidateCacheKey(anime, enabled, includeMikan)
	if cached, ok := o.loadCandidateCache(cacheKey); ok {
		zap.L().Debug("复用 BT 候选缓存",
			zap.Uint("anime_id", anime.ID),
			zap.Int("count", len(cached.Candidates)))
		return cached
	}

	var rssCands []indexer.Candidate
	var rssFailed bool
	var rssSucceeded bool
	if includeMikan && anime.MikanBangumiID != nil && *anime.MikanBangumiID > 0 && o.mikanRSS != nil {
		rssCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		items, err := o.mikanRSS.Fetch(rssCtx, *anime.MikanBangumiID)
		cancel()
		if err != nil {
			rssFailed = true
			zap.L().Warn("Mikan RSS 拉取失败，回退关键词搜索",
				zap.String("anime", anime.Title),
				zap.Int("mikan_bangumi_id", *anime.MikanBangumiID),
				zap.Error(err))
		} else {
			rssSucceeded = true
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
	report := indexer.Aggregate(ctx, enabled, anime.Title)
	kwCands := report.Candidates
	if rssFailed {
		report.FailedSources = append(report.FailedSources, "mikan_rss")
	}
	if rssSucceeded {
		report.SuccessfulSources++
	}

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
	report.Candidates = merged
	o.storeCandidateCache(cacheKey, report)
	return report
}

func (o *Orchestrator) candidateCacheKey(anime *model.Anime, enabled []indexer.Indexer, includeMikan bool) string {
	names := make([]string, 0, len(enabled))
	for _, ix := range enabled {
		names = append(names, ix.Name())
	}
	sort.Strings(names)
	mikanID := 0
	if anime.MikanBangumiID != nil {
		mikanID = *anime.MikanBangumiID
	}
	return fmt.Sprintf("%d|%d|%t|%s|%s", anime.ID, mikanID, includeMikan, anime.Title, strings.Join(names, ","))
}

func (o *Orchestrator) loadCandidateCache(key string) (indexer.SearchResult, bool) {
	o.candidateMu.Lock()
	defer o.candidateMu.Unlock()
	entry, ok := o.candidates[key]
	if !ok {
		return indexer.SearchResult{}, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(o.candidates, key)
		return indexer.SearchResult{}, false
	}
	result := entry.result
	result.Candidates = append([]indexer.Candidate(nil), result.Candidates...)
	result.FailedSources = append([]string(nil), result.FailedSources...)
	return result, true
}

func (o *Orchestrator) storeCandidateCache(key string, result indexer.SearchResult) {
	o.candidateMu.Lock()
	defer o.candidateMu.Unlock()
	// Share incomplete observations briefly across missing episodes without hiding
	// source recovery behind a normal successful-search cache lifetime.
	ttl := candidateCacheTTL
	if len(result.FailedSources) > 0 {
		ttl = time.Minute
	}
	result.Candidates = append([]indexer.Candidate(nil), result.Candidates...)
	result.FailedSources = append([]string(nil), result.FailedSources...)
	o.candidates[key] = candidateCacheEntry{
		result:    result,
		expiresAt: time.Now().Add(ttl),
	}
	// 订阅量通常很小；仍做惰性清理，避免长期改名/切换索引源留下无界缓存。
	if len(o.candidates) > 256 {
		now := time.Now()
		for cacheKey, entry := range o.candidates {
			if now.After(entry.expiresAt) {
				delete(o.candidates, cacheKey)
			}
		}
	}
}

// Run 实现 scheduler.Job 接口：定时对所有订阅番剧逐个跑 CheckAnime。
func (o *Orchestrator) Run(ctx context.Context) {
	o.CheckAllSubscribed(ctx)
}

// CheckAllSubscribed 遍历所有订阅番剧，逐个检查并填坑。
// 同时满足 scheduler.BangumiChecker 接口，可直接替换旧的 bangumi.AutoDownloader。
func (o *Orchestrator) CheckAllSubscribed(ctx context.Context) {
	global, err := LoadGlobal(ctx, o.settingSvc)
	if err != nil {
		zap.L().Error("读取下载策略失败，停止调度", zap.Error(err))
		return
	}

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

// TriggerAutoDownload/TriggerManualCheck 让订阅即时触发和手动检查复用同一套
// Orchestrator 策略。旧 AutoDownloader 会绕过优先级并一次性创建整季流媒体
// 任务，不能再作为这两个入口的执行器。
func (o *Orchestrator) TriggerAutoDownload(ctx context.Context, animeID uint, _ string) {
	o.triggerAnimeCheck(ctx, animeID)
}

func (o *Orchestrator) TriggerManualCheck(ctx context.Context, animeID uint, _ string) {
	o.triggerAnimeCheck(ctx, animeID)
}

func (o *Orchestrator) triggerAnimeCheck(ctx context.Context, animeID uint) {
	if animeID == 0 {
		return
	}
	var anime model.Anime
	if err := o.db.WithContext(ctx).First(&anime, animeID).Error; err != nil {
		zap.L().Warn("orchestrator: 即时检查读取番剧失败", zap.Uint("anime_id", animeID), zap.Error(err))
		return
	}
	o.CheckAnimeWithSettings(ctx, &anime)
}

// CheckAnimeWithSettings loads one consistent settings snapshot before planning.
func (o *Orchestrator) CheckAnimeWithSettings(ctx context.Context, anime *model.Anime) {
	global, err := LoadGlobal(ctx, o.settingSvc)
	if err != nil {
		zap.L().Error("读取下载策略失败，停止调度", zap.Error(err))
		return
	}
	o.CheckAnime(ctx, anime, global)
}

// CheckAnime 检查单个番剧，对缺失集按优先级尝试各源下载。
// global 传入全局偏好；函数内部合并 per-anime override。
func (o *Orchestrator) CheckAnime(ctx context.Context, anime *model.Anime, global Preference) {
	if anime == nil || !anime.IsSubscribed {
		return
	}
	lock := o.animeLock(anime.ID)
	lock.Lock()
	defer lock.Unlock()
	if ctx.Err() != nil {
		return
	}
	pref, err := MergeWithAnime(global, anime)
	if err != nil {
		zap.L().Error("读取番剧下载策略失败，停止调度", zap.Uint("anime_id", anime.ID), zap.Error(err))
		return
	}

	// 期望集数
	expected := 0
	if anime.EpisodeCount != nil {
		expected = *anime.EpisodeCount
	}
	if expected <= 0 {
		o.db.WithContext(ctx).Model(&model.AnimeEpisode{}).
			Where("anime_id = ?", anime.ID).
			Select("COALESCE(MAX(episode_number), 0)").Scan(&expected)
	}
	observedEpisode := 0
	o.db.WithContext(ctx).Model(&model.Download{}).
		Where("anime_id = ? AND episode_number IS NOT NULL", anime.ID).
		Select("COALESCE(MAX(episode_number), 0)").Scan(&observedEpisode)
	if observedEpisode > expected {
		expected = observedEpisode
	}
	if expected <= 0 {
		zap.L().Debug("orchestrator: 番剧无集数信息，跳过", zap.String("title", anime.Title))
		return
	}
	if err := o.checkMediaRootHealth(ctx, o.currentMediaRoot(ctx)); err != nil {
		zap.L().Error("媒体存储不健康，禁止本轮审计和下载", zap.Uint("anime_id", anime.ID), zap.Error(err))
		return
	}
	if o.dlSvc != nil {
		if err := o.dlSvc.ResumeMediaValidation(ctx, anime.ID); err != nil {
			zap.L().Error("恢复暂存媒体验收失败，停止重复选源", zap.Uint("anime_id", anime.ID), zap.Error(err))
			return
		}
	}
	o.auditHistoricalStreamMediaOnce(ctx, anime)

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

	// 每当 Bangumi 剧集表出现更高的已播集数，触发一次持久化水位控制的
	// 当前季文件审计。即使新集已经被 RSS 提前入队，也不会漏掉这次自愈。
	latestAired := latestAiredEpisode(expected, airDates, now)
	triggerEpisode := latestAired
	if observedEpisode > triggerEpisode {
		triggerEpisode = observedEpisode
	}
	state := anime.MediaManagementState
	if state == "" {
		state = MediaStateUninitialized
	}
	// 不依赖可能长期不刷新的 anime.status；当已知总集数中的最终集已经
	// 明确播出，进入最终核验，完整后归档退出目录管理。
	if expected > 0 && latestAired >= expected {
		state = MediaStateFinalizing
	}
	if state == MediaStateArchived || state == MediaStateUninitialized || state == MediaStateFinalizing || triggerEpisode > anime.MediaAuditEpisode {
		result := o.reconcileMissingMedia(ctx, anime)
		if !result.Success {
			return
		}
		if result.Success {
			downloaded = o.downloadedEpisodes(ctx, anime.ID)
			missing = missing[:0]
			skippedUnaired = skippedUnaired[:0]
			for ep := 1; ep <= expected; ep++ {
				if downloaded[ep] {
					continue
				}
				if ad, has := airDates[ep]; has && !episode.IsAired(ad, now) {
					skippedUnaired = append(skippedUnaired, ep)
					continue
				}
				missing = append(missing, ep)
			}
			o.persistMediaAuditResult(ctx, anime, state, triggerEpisode, expected, result)
		}
	}

	quality, qualityErr := o.auditLibraryQuality(ctx, anime, pref)
	if qualityErr != nil {
		zap.L().Warn("剧集质量评估失败，保留现有版本", zap.Uint("anime_id", anime.ID), zap.Error(qualityErr))
		return
	}
	o.persistQualityState(ctx, anime, quality, expected)
	if o.trySeasonUpgrade(ctx, anime, quality, pref) {
		return
	}

	if len(missing) == 0 {
		if len(skippedUnaired) > 0 {
			zap.L().Debug("orchestrator: 番剧无可下载集（剩余均为待发布）",
				zap.String("title", anime.Title),
				zap.Ints("upcoming", skippedUnaired))
		}
		return
	}

	// 已完结/一次性回填大量历史剧集时，先做一次整季级决策。命中合集后
	// 本轮立即结束，绝不再为每一集创建 stream 或第二个几十 GB 的合集。
	if shouldPreferSeasonPack(len(missing), expected) {
		if o.trySeasonPack(ctx, anime, missing, expected, pref) {
			return
		}
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

func (o *Orchestrator) animeLock(id uint) *sync.Mutex {
	lock, _ := o.animeLocks.LoadOrStore(id, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func latestAiredEpisode(expected int, airDates map[int]string, now time.Time) int {
	latest := 0
	for ep := 1; ep <= expected; ep++ {
		airDate, has := airDates[ep]
		if !has || airDate == "" {
			continue
		}
		if !episode.IsAired(airDate, now) {
			continue
		}
		latest = ep
	}
	return latest
}

// tryDownloadEpisode 严格按优先级逐级降级。一个来源成功入队后本轮立即停止；
// 只有候选失败、判死种或进入 seeking_alternative 后，下一轮才尝试同层下一
// 候选或更低优先级来源。这样“优先级”才是用户理解的选择顺序，而不是把
// Mikan、普通 BT、流媒体全部同时下载。
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
		case SourceBT, SourceMikan:
			ok, diagResultCount, diagRankedOut, diagReason, diagBestTitle, diagBestScore =
				o.tryBT(ctx, anime, ep, pref, retryCount, srcType, false)
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

func shouldPreferSeasonPack(missingCount, expected int) bool {
	if missingCount < 4 || expected <= 0 {
		return false
	}
	return missingCount*100 >= expected*30
}

type seasonPackCandidate struct {
	Candidate indexer.ScoredCandidate
	Start     int
	End       int
	Coverage  int
}

// trySeasonPack 从最高优先级的 BT 层开始找一个覆盖大部分缺集的合集。
// 每轮最多入队一个合集；没有合格合集时才回到逐集调度。
func (o *Orchestrator) trySeasonPack(ctx context.Context, anime *model.Anime, missing []int, expected int, pref Preference) bool {
	if o.dlSvc == nil || !o.dlSvc.HasExecutor(model.DownloadTypeTorrent) || len(missing) == 0 {
		return false
	}
	firstMissing := missing[0]
	minimumCoverage := (len(missing)*70 + 99) / 100
	if minimumCoverage < 4 {
		minimumCoverage = 4
	}
	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	}
	if namedSeason := titleparse.InstallmentNumber(anime.Title); namedSeason > 1 {
		season = namedSeason
	}

	for _, tier := range pref.Priority {
		if tier != SourceMikan && tier != SourceBT {
			continue
		}
		if pref.IsSourceDisabled(tier) {
			continue
		}
		enabled := o.enabledIndexersForTier(pref, tier)
		if len(enabled) == 0 {
			continue
		}
		search := o.collectBTCandidates(ctx, anime, enabled, tier == SourceMikan)
		cands := search.Candidates
		ranked := indexer.RankByPreferenceAndSeason(filterInstallmentCandidates(anime.Title, cands), pref.ToIndexerPref(), 0, season)
		ranked = retainPreferredLanguage(ranked, pref.Languages)
		packs := make([]seasonPackCandidate, 0)
		for _, candidate := range ranked {
			if candidate.Parsed == nil || !candidate.Parsed.IsBatch || candidate.Parsed.BatchStart == nil || candidate.Parsed.BatchEnd == nil {
				continue
			}
			start, end := *candidate.Parsed.BatchStart, *candidate.Parsed.BatchEnd
			if end == expected-1 && containsExtraEpisode(candidate.Title) {
				end = expected
			}
			coverage := countEpisodesInRange(missing, start, end)
			if coverage < minimumCoverage {
				continue
			}
			url := candidate.MagnetURL
			if url == "" {
				url = candidate.TorrentURL
			}
			if url == "" || o.isDuplicateURL(ctx, anime.ID, url) ||
				(candidate.InfoHash != "" && (o.hasActiveInfoHash(ctx, anime.ID, candidate.InfoHash) ||
					o.hasHistoricalFailure(ctx, candidate.InfoHash) || o.hasAnimeInfoHashFailure(ctx, anime.ID, candidate.InfoHash))) ||
				o.hasAnimeURLFailure(ctx, anime.ID, url) {
				continue
			}
			packs = append(packs, seasonPackCandidate{Candidate: candidate, Start: start, End: end, Coverage: coverage})
		}
		if len(packs) == 0 {
			o.recordDiag(anime.ID, firstMissing, tier, len(cands), len(cands),
				"未找到覆盖至少 70% 缺集的健康合集，降级继续检索", "", 0)
			continue
		}
		sort.SliceStable(packs, func(i, j int) bool {
			if packs[i].Coverage != packs[j].Coverage {
				return packs[i].Coverage > packs[j].Coverage
			}
			return packs[i].Candidate.Score > packs[j].Candidate.Score
		})
		top := packs[0]
		url := top.Candidate.MagnetURL
		if url == "" {
			url = top.Candidate.TorrentURL
		}
		start, end := top.Start, top.End
		ep := firstMissing
		task := &dlservice.Task{
			Name:           fmt.Sprintf("%s - 合集 %02d-%02d", anime.Title, start, end),
			URL:            url,
			SourceTitle:    top.Candidate.Title,
			EstimatedBytes: top.Candidate.Size,
			DownloadType:   model.DownloadTypeTorrent,
			Source:         tier,
			AnimeName:      anime.Title,
			AnimeID:        &anime.ID,
			EpisodeNumber:  &ep,
			Scope:          model.DownloadScopeSeason,
			EpisodeStart:   &start,
			EpisodeEnd:     &end,
			RetryCount:     o.retryCountForEpisode(ctx, anime.ID, firstMissing),
		}
		if _, err := o.dlSvc.Create(ctx, task); err != nil {
			o.recordDiag(anime.ID, firstMissing, tier, len(cands), len(cands)-len(packs),
				"创建合集任务失败: "+err.Error(), top.Candidate.Title, top.Candidate.Score)
			continue
		}
		reason := fmt.Sprintf("整季优先：已入队覆盖 %d/%d 个缺集的合集 %02d-%02d", top.Coverage, len(missing), start, end)
		o.recordDiag(anime.ID, firstMissing, tier, len(cands), len(cands)-len(packs), reason, top.Candidate.Title, top.Candidate.Score)
		zap.L().Info("orchestrator: 整季合集优先入队",
			zap.String("anime", anime.Title), zap.String("source", tier),
			zap.Int("start", start), zap.Int("end", end), zap.Int("coverage", top.Coverage),
			zap.String("title", top.Candidate.Title))
		return true
	}
	return false
}

func (o *Orchestrator) enabledIndexersForTier(pref Preference, tier string) []indexer.Indexer {
	enabled := make([]indexer.Indexer, 0, len(pref.EnabledIndexers))
	for _, name := range pref.EnabledIndexers {
		isMikan := name == SourceMikan
		if (tier == SourceMikan) != isMikan {
			continue
		}
		if ix, has := o.indexers[name]; has {
			enabled = append(enabled, ix)
		}
	}
	return enabled
}

func countEpisodesInRange(episodes []int, start, end int) int {
	count := 0
	for _, ep := range episodes {
		if ep >= start && ep <= end {
			count++
		}
	}
	return count
}

func containsExtraEpisode(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "ova") || strings.Contains(lower, "oad") || strings.Contains(lower, "oav")
}

// ---- Stream 源适配 ----

func isRuleBroken(rule *model.StreamRule) bool {
	if rule.HealthStatus == nil || *rule.HealthStatus != "broken" {
		return false
	}
	// broken 规则冷却 1 小时后进入 half-open，由一次真实任务探测是否恢复。
	return rule.HealthAt != nil && time.Since(*rule.HealthAt) < time.Hour
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

func (o *Orchestrator) tryBT(
	ctx context.Context,
	anime *model.Anime,
	ep int,
	pref Preference,
	retryCount int,
	tier string,
	allowAdditional bool,
) (
	ok bool, resultCount int, rankedOut int, reason, bestTitle string, bestScore float64,
) {
	if o.dlSvc == nil || !o.dlSvc.HasExecutor(model.DownloadTypeTorrent) {
		return false, 0, 0, "qBittorrent 当前不可用，跳过 BT 入队", "", 0
	}
	if !allowAdditional && o.isDuplicate(ctx, anime.ID, ep, tier) {
		return false, 0, 0, "该来源已有下载记录（同集），跳过", "", 0
	}
	// 选启用的 indexer
	enabled := o.enabledIndexersForTier(pref, tier)
	if len(enabled) == 0 {
		if tier == SourceMikan {
			return false, 0, 0, "Mikan 未启用", "", 0
		}
		return false, 0, 0, "无启用的普通 BT indexer", "", 0
	}

	search := o.collectBTCandidates(ctx, anime, enabled, tier == SourceMikan)
	cands := search.Candidates
	defer func() {
		if resultCount > 0 && search.Warning() != "" {
			reason += "；" + search.Warning()
		}
	}()
	resultCount = len(cands)
	if resultCount == 0 {
		if warning := search.Warning(); warning != "" {
			return false, 0, 0, warning, "", 0
		}
		return false, 0, 0, "所有已查询索引均无结果", "", 0
	}

	season := 1
	if anime.Season != nil && *anime.Season > 0 {
		season = *anime.Season
	}
	if namedSeason := titleparse.InstallmentNumber(anime.Title); namedSeason > 1 {
		season = namedSeason
	}
	ranked := indexer.RankByPreferenceAndSeason(filterInstallmentCandidates(anime.Title, cands), pref.ToIndexerPref(), ep, season)
	rankedOut = resultCount - len(ranked)
	if len(ranked) == 0 {
		return false, resultCount, rankedOut, "所有候选均不符合偏好（集数不匹配/分辨率不符/字幕组不符）", "", 0
	}
	preferred := retainPreferredLanguage(ranked, pref.Languages)
	rankedOut += len(ranked) - len(preferred)
	ranked = preferred

	// 否决黑名单以及本番已经失败过的 InfoHash。必须在选 top 之前过滤，
	// 否则第一名失败后会每轮都再次选中第一名，并在后面的去重检查处停止，
	// 永远轮不到第二候选。
	rankedFiltered := ranked[:0]
	for _, c := range ranked {
		candidateURL := c.MagnetURL
		if candidateURL == "" {
			candidateURL = c.TorrentURL
		}
		if candidateURL == "" {
			rankedOut++
			continue
		}
		if c.InfoHash != "" && (o.hasActiveInfoHash(ctx, anime.ID, c.InfoHash) ||
			o.hasHistoricalFailure(ctx, c.InfoHash) ||
			o.hasAnimeInfoHashFailure(ctx, anime.ID, c.InfoHash)) {
			rankedOut++
			continue
		}
		// torrent URL 不一定能提前提取 InfoHash。失败/淘汰过的 URL 也必须
		// 在选 top 前剔除，否则 isDuplicateURL 会把它误报成“合集已入队”，
		// 整轮提前结束，后面的候选永远没有机会。
		if o.hasAnimeURLFailure(ctx, anime.ID, candidateURL) {
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
		index   int
		c       indexer.ScoredCandidate
		ok      bool // scrape 收到回包
		seeders int
	}
	scrapedCh := make(chan scrapedCand, len(ranked))
	var swg sync.WaitGroup
	for i := range ranked {
		index := i
		c := ranked[i]
		magnet := c.MagnetURL
		if magnet == "" {
			magnet = c.InfoHash
		}
		if magnet == "" {
			scrapedCh <- scrapedCand{index: index, c: c, ok: false} // 缺 hash 不能 scrape，保留
			continue
		}
		swg.Add(1)
		go func() {
			defer swg.Done()
			r := indexer.ScrapeMagnet(scrapeCtx, magnet)
			scrapedCh <- scrapedCand{index: index, c: c, ok: r.OK, seeders: r.Seeders}
		}()
	}
	go func() { swg.Wait(); close(scrapedCh); cancel() }()

	scraped := make([]scrapedCand, len(ranked))
	scrapedDead := 0
	scrapedNoResponse := 0
	for sc := range scrapedCh {
		scraped[sc.index] = sc
		switch {
		case sc.ok && sc.seeders > 0:
			// 探活成功且有活种
			sc.c.Seeders = sc.seeders
			sc.c.SeedersReported = true
			scraped[sc.index] = sc
		case sc.ok && sc.seeders == 0:
			// 探活成功但无活种 → 死种
			scrapedDead++
		default:
			// scrape 没回包：保留（fail-open）
			scrapedNoResponse++
		}
	}
	// 并发探测的返回顺序不稳定，必须按原评分顺序重建列表。否则响应最快的
	// 低分候选会随机抢到第一名，破坏 RankByPreferenceAndSeason 的结果。
	alive := make([]indexer.ScoredCandidate, 0, len(ranked))
	for _, sc := range scraped {
		if sc.ok && sc.seeders == 0 {
			continue
		}
		alive = append(alive, sc.c)
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

	// 去重 1：同一个 anime 下同一 URL 已经提交过（批量包场景：01-12 Fin 不要为每集重复入队）
	if o.isDuplicateURL(ctx, anime.ID, url) {
		if allowAdditional {
			return false, resultCount, rankedOut, "合集种子已经覆盖当前集，无需重复补位", bestTitle, bestScore
		}
		return true, resultCount, rankedOut, "合集种子已入队（当前集在批量包内）", bestTitle, bestScore
	}
	// 去重 2：同 anime 下同 InfoHash 已经存在记录（含 failed/completed），不论 source/episode。
	// 防止 sync 误标 failed 后 Orchestrator 再次拿同 magnet 入队产生双胞胎。
	if o.isDuplicateInfoHash(ctx, anime.ID, top.InfoHash) {
		return false, resultCount, rankedOut, "该 InfoHash 已存在历史记录，跳过", bestTitle, bestScore
	}
	activeEpisodeTorrents := o.activeEpisodeTorrentCount(ctx, anime.ID, ep)
	if activeEpisodeTorrents >= maxConcurrentEpisodeTorrents {
		return false, resultCount, rankedOut,
			fmt.Sprintf("同集已有 %d 个候选并行探测，达到安全上限", activeEpisodeTorrents),
			bestTitle, bestScore
	}

	// 入队（使用 magnet，走 torrent 执行器）
	epCopy := ep

	task := &dlservice.Task{
		Name:           fmt.Sprintf("%s - 第%02d集", anime.Title, ep),
		URL:            url,
		SourceTitle:    top.Title,
		EstimatedBytes: top.Size,
		DownloadType:   model.DownloadTypeTorrent,
		Source:         tier,
		AnimeName:      anime.Title,
		AnimeID:        &anime.ID,
		EpisodeNumber:  &epCopy,
		SavePath: dlservice.BuildTorrentRaceSavePath(
			o.currentMediaRoot(ctx), anime.ID, ep, top.InfoHash, url),
		RetryCount: retryCount,
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

// retainPreferredLanguage turns an explicit language preference into a hard
// race boundary whenever matching candidates exist. This prevents a faster
// CHT-only release from beating an available CHS release.
func retainPreferredLanguage(ranked []indexer.ScoredCandidate, preferred []string) []indexer.ScoredCandidate {
	if len(ranked) == 0 || len(preferred) == 0 {
		return ranked
	}
	wanted := make(map[string]bool, len(preferred))
	for _, lang := range preferred {
		lang = strings.TrimSpace(strings.ToLower(lang))
		if lang != "" {
			wanted[lang] = true
		}
	}
	if len(wanted) == 0 {
		return ranked
	}
	hasMatch := false
	for _, candidate := range ranked {
		if candidateHasLanguage(candidate, wanted) {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		return ranked
	}
	out := make([]indexer.ScoredCandidate, 0, len(ranked))
	for _, candidate := range ranked {
		if candidateHasLanguage(candidate, wanted) {
			out = append(out, candidate)
		}
	}
	return out
}

func candidateHasLanguage(candidate indexer.ScoredCandidate, wanted map[string]bool) bool {
	if candidate.Parsed == nil {
		return false
	}
	for _, lang := range candidate.Parsed.Lang {
		if wanted[strings.ToLower(lang)] {
			return true
		}
	}
	return false
}

// currentMediaRoot keeps BT output aligned with the download directory changed
// from Settings. The configured process default is only a fallback.
func (o *Orchestrator) currentMediaRoot(ctx context.Context) string {
	if o.settingSvc != nil {
		if value, ok, err := o.settingSvc.Get(ctx, "download_dir"); err == nil && ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return o.mediaRoot
}

// ---- RSS 源说明 ----
// RSS 不再是 orchestrator 的主动源。被动通道由 RSSRefreshJob + rssrule 处理：
// 定时刷新 feeds → 解析入 rssentry 表 → 命中规则即下载。
// 若需要诊断 RSS 命中情况，请查 rssentry 表，不要在 tryDownloadEpisode 里再开一档。

// ---- 辅助 ----

// retryCountForEpisode calculates the next actual attempt number. Discovery
// without enqueueing does not consume budget. Rejected candidates carry the
// existing budget but do not consume another attempt by themselves.
func (o *Orchestrator) retryCountForEpisode(ctx context.Context, animeID uint, ep int) int {
	var count int
	o.db.WithContext(ctx).Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND status = ?", animeID, ep, model.DownloadStatusFailed).
		Where("failure_kind IN ?", []string{model.FailureKindTransient, model.FailureKindExhausted}).
		Select("COALESCE(MAX(retry_count + 1), 0)").Scan(&count)
	if count > 3 {
		return 3
	}
	return count
}

// selectStreamEpisode 按线路组织扁平剧集列表，并选择指定集。
// 首选线路 → 其他线路；冷却中的线路绝不兜回，等待下一次探测。
func selectStreamEpisode(episodes []stream.EpisodeInfo, ep int, preferred string, failed map[string]bool, multi bool) (stream.EpisodeInfo, string, bool) {
	if ep <= 0 || failed["*"] {
		return stream.EpisodeInfo{}, "", false
	}
	byRoad := make(map[string][]stream.EpisodeInfo)
	order := make([]string, 0)
	for _, road := range stream.EpisodeRoads(episodes, 0) {
		order = append(order, road.Name)
		byRoad[road.Name] = road.Episodes
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
	for _, road := range order {
		if failed[road] {
			continue
		}
		if item, ok := findEpisode(byRoad[road], ep); ok {
			return item, road, true
		}
	}
	return stream.EpisodeInfo{}, "", false
}

func parseEpisodeNumber(item stream.EpisodeInfo) (int, bool) {
	return stream.EpisodeNumber(item)
}

func findEpisode(items []stream.EpisodeInfo, ep int) (stream.EpisodeInfo, bool) {
	return stream.FindEpisode(items, ep)
}

func hasEpisode(episodes []stream.EpisodeInfo, ep int) bool {
	_, _, ok := selectStreamEpisode(episodes, ep, "", nil, true)
	return ok
}

func firstRoadWithEpisode(episodes []stream.EpisodeInfo, ep int) string {
	_, road, _ := selectStreamEpisode(episodes, ep, "", nil, true)
	return road
}

const maxConcurrentEpisodeTorrents = 3

func (o *Orchestrator) activeEpisodeTorrentCount(ctx context.Context, animeID uint, ep int) int64 {
	var count int64
	o.db.WithContext(ctx).Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND download_type = ? AND status IN ?",
			animeID, ep, model.DownloadTypeTorrent,
			[]string{model.DownloadStatusPending, model.DownloadStatusDownloading}).
		Count(&count)
	return count
}

func (o *Orchestrator) hasActiveSourceCandidate(ctx context.Context, animeID uint, ep int, source string) bool {
	var count int64
	o.db.WithContext(ctx).Model(&model.Download{}).
		Where("anime_id = ? AND episode_number = ? AND source = ? AND status IN ?",
			animeID, ep, source,
			[]string{model.DownloadStatusPending, model.DownloadStatusDownloading}).
		Count(&count)
	return count > 0
}

func (o *Orchestrator) activeEpisodeRaces(ctx context.Context, animeID uint) []int {
	var episodes []int
	o.db.WithContext(ctx).Model(&model.Download{}).
		Where("anime_id = ? AND episode_number IS NOT NULL AND status IN ?",
			animeID, []string{model.DownloadStatusPending, model.DownloadStatusDownloading}).
		Distinct("episode_number").
		Pluck("episode_number", &episodes)
	var completed []int
	o.db.WithContext(ctx).Model(&model.Download{}).
		Where("anime_id = ? AND episode_number IS NOT NULL AND status = ?",
			animeID, model.DownloadStatusCompleted).
		Distinct("episode_number").
		Pluck("episode_number", &completed)
	done := make(map[int]bool, len(completed))
	for _, ep := range completed {
		done[ep] = true
	}
	filtered := episodes[:0]
	for _, ep := range episodes {
		if !done[ep] {
			filtered = append(filtered, ep)
		}
	}
	sort.Ints(filtered)
	return filtered
}

func (o *Orchestrator) episodeRaceNeedsFill(ctx context.Context, animeID uint, ep int, pref Preference) bool {
	if o.activeEpisodeTorrentCount(ctx, animeID, ep) < maxConcurrentEpisodeTorrents {
		return true
	}
	return !pref.IsSourceDisabled(SourceStream) &&
		!o.hasActiveSourceCandidate(ctx, animeID, ep, SourceStream)
}

// downloadedEpisodes 查询某 anime 的已完成/进行中集数（跨所有 source_type）。
// 已标记 seeking_alternative 的慢种仍在 qBit 中继续下载，但不再独占该集，
// 这样调度器可以并行加入不同 Hash。
func (o *Orchestrator) downloadedEpisodes(ctx context.Context, animeID uint) map[int]bool {
	var rows []struct {
		EpisodeNumber      *int
		Scope              string
		EpisodeStart       *int
		EpisodeEnd         *int
		Status             string
		MediaMissing       bool
		SeekingAlternative bool
	}
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ?", animeID).
		Where("status IN ?", []string{
			model.DownloadStatusCompleted,
			model.DownloadStatusDownloading,
			model.DownloadStatusPending,
			model.DownloadStatusPaused,
		}).
		Select("episode_number, scope, episode_start, episode_end, status, media_missing, seeking_alternative").
		Scan(&rows)

	out := make(map[int]bool, len(rows))
	for _, r := range rows {
		if r.MediaMissing || (r.SeekingAlternative && r.Status != model.DownloadStatusPaused) {
			continue
		}
		// 下载中的合集暂时覆盖整个范围，防止它下载时又启动逐集任务。
		// 完成后的真实覆盖范围由 AnimeEpisode 文件映射决定，不能仅凭标题
		// 声明把无法识别或缺失的文件误标为完成。
		if r.Scope == model.DownloadScopeSeason && r.Status != model.DownloadStatusCompleted &&
			r.EpisodeStart != nil && r.EpisodeEnd != nil {
			for ep := *r.EpisodeStart; ep <= *r.EpisodeEnd; ep++ {
				out[ep] = true
			}
			continue
		}
		if r.Scope == model.DownloadScopeSeason {
			continue // Completed packs are represented only by mapped episode files.
		}
		if r.EpisodeNumber != nil {
			out[*r.EpisodeNumber] = true
		}
	}
	var diskRows []model.AnimeEpisode
	o.db.WithContext(ctx).Where("anime_id = ? AND downloaded = ?", animeID, true).Find(&diskRows)
	for _, row := range diskRows {
		out[row.EpisodeNumber] = true
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

// isDuplicate protects active/paused work and applies source-level cooldown.
// Only the latest failed attempt controls that cooldown; historical failures
// retain candidate diagnostics but must not extend a newer attempt's backoff.
func (o *Orchestrator) isDuplicate(ctx context.Context, animeID uint, ep int, sourceType string) bool {
	sources := []string{sourceType}
	if sourceType == SourceStream {
		sources = append(sources, dlservice.SourceBangumi)
	}
	var count int64
	base := func() *gorm.DB {
		return o.db.WithContext(ctx).Model(&model.Download{}).
			Where("anime_id = ? AND episode_number = ? AND source IN ?", animeID, ep, sources)
	}
	if err := base().
		Where("status IN ? OR (status = ? AND media_missing = ?)",
			[]string{model.DownloadStatusDownloading, model.DownloadStatusPending, model.DownloadStatusPaused},
			model.DownloadStatusCompleted, false).
		Where("seeking_alternative = ? OR status IN ?", false,
			[]string{model.DownloadStatusCompleted, model.DownloadStatusPaused}).Count(&count).Error; err != nil {
		return true // Fail closed on DB errors; the next scan can try again.
	}
	if count > 0 {
		return true
	}
	var failure model.Download
	err := base().Where("status = ?", model.DownloadStatusFailed).Order("id DESC").Take(&failure).Error
	if err != nil {
		return err != gorm.ErrRecordNotFound
	}
	switch failure.FailureKind {
	case model.FailureKindTransient:
		if sourceType == SourceStream {
			return false
		} // Per-road cooldown must not block healthy backup roads.
		return failure.NextRetryAt != nil && time.Now().Before(*failure.NextRetryAt)
	case model.FailureKindPermanent, "":
		return time.Since(failure.UpdatedAt) < 6*time.Hour
	case model.FailureKindExhausted:
		if sourceType == SourceStream {
			return false
		}
		return time.Since(failure.UpdatedAt) < time.Hour
	default:
		return false // Rejected candidates are filtered individually, not by source.
	}
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

func (o *Orchestrator) hasActiveInfoHash(ctx context.Context, animeID uint, infoHash string) bool {
	if infoHash == "" {
		return false
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND info_hash = ? AND status IN ?",
			animeID, strings.ToUpper(infoHash),
			[]string{model.DownloadStatusPending, model.DownloadStatusDownloading}).
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
	var row model.AbandonedTorrent
	if err := o.db.WithContext(ctx).
		Where("info_hash = ?", strings.ToUpper(infoHash)).
		First(&row).Error; err != nil {
		return false
	}
	if row.Kind == model.FailureKindPermanent {
		return true
	}
	cooldown := 72 * time.Hour
	if strings.Contains(row.Reason, "慢种") || strings.Contains(row.Reason, "平均速度") {
		cooldown = 12 * time.Hour
	}
	return time.Since(row.AbandonedAt) < cooldown
}

// hasAnimeInfoHashFailure 判断该番是否已经尝试并失败/淘汰过某个种子。
// 与全局 abandoned_torrent 不同，它只影响当前番，避免解析误关联污染其他番剧。
func (o *Orchestrator) hasAnimeInfoHashFailure(ctx context.Context, animeID uint, infoHash string) bool {
	if infoHash == "" {
		return false
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND info_hash = ? AND status IN ?",
			animeID, strings.ToUpper(infoHash), []string{
				model.DownloadStatusFailed,
				model.DownloadStatusSuperseded,
			}).
		Count(&count)
	return count > 0
}

func (o *Orchestrator) hasAnimeURLFailure(ctx context.Context, animeID uint, candidateURL string) bool {
	if candidateURL == "" {
		return false
	}
	var count int64
	o.db.WithContext(ctx).
		Model(&model.Download{}).
		Where("anime_id = ? AND url = ? AND status IN ?",
			animeID, candidateURL, []string{
				model.DownloadStatusFailed,
				model.DownloadStatusSuperseded,
			}).
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

func filterInstallmentCandidates(title string, candidates []indexer.Candidate) []indexer.Candidate {
	filtered := make([]indexer.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if titleparse.InstallmentCompatible(title, candidate.Title) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}
