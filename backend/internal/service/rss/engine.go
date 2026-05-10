package rss

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	bangumisvc "github.com/anidog/anidog-go/internal/service"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

// Engine fetches RSS feeds, matches entries against rules, and triggers downloads.
type Engine struct {
	db         *gorm.DB
	dlSvc      dlservice.Downloader
	bangumiSvc *bangumisvc.BangumiService // 可选；用于自动发现时补全元数据
	parser     *Parser
	matcher    *Matcher
	mediaRoot  string // 下载根目录，用于按番剧组织子目录
}

// NewEngine creates a new RSS engine.
func NewEngine(db *gorm.DB, dlSvc dlservice.Downloader) *Engine {
	return &Engine{
		db:      db,
		dlSvc:   dlSvc,
		parser:  NewParser(),
		matcher: NewMatcher(),
	}
}

// SetBangumiService 注入 Bangumi 服务，启用后自动发现时会补全元数据。
func (e *Engine) SetBangumiService(svc *bangumisvc.BangumiService) {
	e.bangumiSvc = svc
}

// SetMediaRoot 设置下载根目录，用于生成番剧子目录路径
func (e *Engine) SetMediaRoot(root string) {
	e.mediaRoot = root
}

// CheckFeed fetches and processes a single RSS feed.
func (e *Engine) CheckFeed(ctx context.Context, feed *model.RSSFeed) (int, error) {
	if !feed.Enabled {
		return 0, nil
	}

	items, err := e.parser.Parse(ctx, feed.URL)
	if err != nil {
		return 0, err
	}

	// Load rules for this feed
	var rules []model.RSSRule
	e.db.Where("rss_feed_id = ?", feed.ID).Find(&rules)

	newCount := 0
	for _, item := range items {
		// Dedup: check if we've already seen this entry
		var existing model.RSSEntry
		if e.db.Where("entry_id = ? AND rss_feed_id = ?", item.EntryID, feed.ID).First(&existing).Error == nil {
			continue
		}

		// 解析标题提取字幕组/番名/集数
		parsed := titleparse.Parse(item.Title)

		// Record the entry（含解析结果）
		entry := model.RSSEntry{
			EntryID:       item.EntryID,
			Title:         item.Title,
			Link:          item.Link,
			Published:     item.Published,
			RSSFeedID:     feed.ID,
			ParsedEpisode: parsed.EpisodeNum,
		}
		if parsed.AnimeName != "" {
			entry.ParsedAnime = &parsed.AnimeName
		}
		if parsed.Group != "" {
			entry.ParsedGroup = &parsed.Group
		}

		// 关联已订阅番剧（常规路径）
		var matchedAnime *model.Anime
		if parsed.AnimeName != "" {
			matchedAnime = e.findSubscribedAnime(parsed.AnimeName)
		}
		if matchedAnime == nil {
			matchedAnime = e.findByTitleSubstring(item.Title)
		}

		// 自动发现模式：没匹配到已追番剧时，创建新的 anime 条目
		if matchedAnime == nil && feed.AutoDiscover && parsed.AnimeName != "" {
			matchedAnime = e.autoDiscoverAnime(ctx, parsed.AnimeName, parsed.AltNames)
		}

		if matchedAnime != nil {
			entry.MatchedAnimeID = &matchedAnime.ID
		}
		e.db.Create(&entry)

		// Match against rules
		if !e.matcher.Match(item.Title, rules) {
			continue
		}

		// Create download task via the unified service
		task := &dlservice.Task{
			Name:         item.Title,
			URL:          item.Link,
			DownloadType: model.DownloadTypeTorrent,
			Source:       dlservice.SourceRSS,
		}
		if matchedAnime != nil {
			task.AnimeID = &matchedAnime.ID
			task.AnimeName = matchedAnime.Title
			// 按番剧名组织子目录
			task.SavePath = dlservice.BuildAnimeSavePath(e.mediaRoot, matchedAnime)
		}
		if parsed.EpisodeNum != nil {
			task.EpisodeNumber = parsed.EpisodeNum
		}

		if _, err := e.dlSvc.Create(ctx, task); err != nil {
			zap.L().Error("RSS 创建下载任务失败", zap.String("title", item.Title), zap.Error(err))
			continue
		}

		// Mark entry as downloaded
		e.db.Model(&entry).Update("downloaded", true)
		newCount++
	}

	// Update last check time
	now := time.Now()
	e.db.Model(feed).Update("last_check", &now)

	zap.L().Info("RSS 检查完成", zap.String("feed", feed.Name), zap.Int("new", newCount), zap.Int("total", len(items)))
	return newCount, nil
}

// RefreshAll checks all enabled RSS feeds.
func (e *Engine) RefreshAll(ctx context.Context) {
	var feeds []model.RSSFeed
	e.db.Where("enabled = ?", true).Find(&feeds)

	zap.L().Info("开始刷新所有 RSS 订阅源", zap.Int("count", len(feeds)))

	for i := range feeds {
		if _, err := e.CheckFeed(ctx, &feeds[i]); err != nil {
			zap.L().Error("RSS 检查失败", zap.String("feed", feeds[i].Name), zap.Error(err))
		}
	}
}

// ParseFeedPreview parses a feed URL without creating downloads (for preview/test).
func (e *Engine) ParseFeedPreview(ctx context.Context, feedURL string) ([]ParsedItem, error) {
	return e.parser.Parse(ctx, feedURL)
}

// findSubscribedAnime 按解析的番名在订阅番剧里精确/模糊匹配
func (e *Engine) findSubscribedAnime(name string) *model.Anime {
	if name == "" {
		return nil
	}
	var animes []model.Anime
	e.db.Where("is_subscribed = ?", true).Find(&animes)
	for i := range animes {
		a := &animes[i]
		if a.Title == name {
			return a
		}
		if containsMatch(name, a.Title) || containsMatch(a.Title, name) {
			return a
		}
	}
	return nil
}

// findByTitleSubstring 原有的松散匹配（兼容老逻辑）
func (e *Engine) findByTitleSubstring(title string) *model.Anime {
	var animes []model.Anime
	e.db.Where("is_subscribed = ?", true).Find(&animes)
	for i := range animes {
		a := &animes[i]
		if a.Title != "" && containsMatch(title, a.Title) {
			return a
		}
	}
	return nil
}

// autoDiscoverAnime 自动创建未订阅番剧条目（AutoBangumi 模式）
// 优先走 Bangumi search 拿到完整元数据；失败时 fallback 到最小记录。
// 原则：
//   1) title 始终用 parsed.AnimeName（保留"第X季"后缀，避免把 S5 创建成 S1）
//   2) 去重按 title 精确匹配（parsed_anime 相同视为同一部）
//   3) Bangumi 元数据尽力拿：先精确季度，再系列兜底，用于海报/评分等
func (e *Engine) autoDiscoverAnime(ctx context.Context, name string, alts []string) *model.Anime {
	// 去重：按 parsed name 精确匹配
	var existing model.Anime
	if err := e.db.Where("title = ?", name).First(&existing).Error; err == nil {
		if !existing.IsSubscribed {
			e.db.Model(&existing).Updates(map[string]interface{}{
				"is_subscribed": true,
				"source_origin": "rss_auto",
			})
			existing.IsSubscribed = true
		}
		// 元数据缺失时补全
		if existing.EpisodeCount == nil || existing.CoverURL == nil {
			e.enrichFromBangumi(ctx, &existing)
		}
		return &existing
	}

	// 尝试用 Bangumi 搜索（支持多候选名 + 季度感知）
	detail := e.searchBangumi(ctx, name, alts)

	newAnime := model.Anime{
		Title:        name, // 始终用 parsed.AnimeName，保留季度后缀
		IsSubscribed: true,
		Status:       model.AnimeStatusUnknown,
		SourceOrigin: "rss_auto",
	}
	if detail != nil {
		bangumiID := detail.ID
		newAnime.BangumiID = &bangumiID
		newAnime.OriginalTitle = &detail.Name
		// 不覆盖 title（保持 parsed name）；把 NameCN 放 original_title 参考
		if detail.Summary != "" {
			newAnime.Description = &detail.Summary
		}
		if detail.ImageURL != "" {
			newAnime.CoverURL = &detail.ImageURL
		}
		if detail.Rating > 0 {
			newAnime.BangumiRating = &detail.Rating
		}
		if detail.EpsCount > 0 {
			newAnime.EpisodeCount = &detail.EpsCount
		}
		if detail.AirWeekday >= 0 {
			newAnime.AirWeekday = &detail.AirWeekday
		}
		if y := yearFromAirDate(detail.AirDate); y > 0 {
			newAnime.Year = &y
		}
		newAnime.Status = computeStatusFromAirDate(detail.AirDate, detail.EpsCount)
		if newAnime.Status == "" {
			newAnime.Status = model.AnimeStatusUnknown
		}
	}

	if err := e.db.Create(&newAnime).Error; err != nil {
		zap.L().Warn("RSS 自动发现：创建番剧失败", zap.String("name", name), zap.Error(err))
		return nil
	}
	zap.L().Info("RSS 自动发现：新增追番",
		zap.String("name", newAnime.Title),
		zap.Uint("anime_id", newAnime.ID),
		zap.Bool("has_metadata", detail != nil))
	return &newAnime
}

// reSeasonFromName 从 parsed name 提取季度数字。
// 匹配: "第二季" / "第2季" / "第Ⅱ季" / "Season 2" / "S2" / "2nd Season" / "第四季"
var reSeasonFromName = regexp.MustCompile(`(?i)(?:第\s*([二三四五六七八九十\d]+)\s*[季部期]|S(?:eason)?\s*(\d+)|(\d+)(?:st|nd|rd|th)\s*Season|(\d+)(?:st|nd|rd|th)\s*Period)`)

// reStripSeason 删除 parsed name 里的"第X季"/"第X期"/"SN"等季度后缀，得到"基础名"
var reStripSeason = regexp.MustCompile(`(?i)\s*(?:第\s*[二三四五六七八九十\d]+\s*[季部期]|S(?:eason)?\s*\d+|\d+(?:st|nd|rd|th)\s*Season|\d+(?:st|nd|rd|th)\s*Period)\s*`)

// stripSeasonSuffix "出租女友 第五季" → "出租女友"
func stripSeasonSuffix(name string) string {
	s := reStripSeason.ReplaceAllString(name, " ")
	return strings.TrimSpace(s)
}

func extractSeasonFromName(name string) int {
	if name == "" {
		return 0
	}
	m := reSeasonFromName.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	for i := 1; i < len(m); i++ {
		if m[i] == "" {
			continue
		}
		if n := parseChineseOrInt(m[i]); n > 0 {
			return n
		}
	}
	return 0
}

// parseChineseOrInt "二" → 2, "12" → 12
func parseChineseOrInt(s string) int {
	cn := map[string]int{"一": 1, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9, "十": 10}
	if n, ok := cn[s]; ok {
		return n
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// searchBangumi 用多候选名 + 季度感知的方式找到最佳 Bangumi 条目。
// 策略：
//   1) 候选关键词 = [parsed.AnimeName, ...AltNames, stripSeasonSuffix(AnimeName), stripSeasonSuffix(每个 Alt)]
//   2) 挨个搜，汇总所有结果去重
//   3) 若需要 season>1：找 name/name_cn/原名 含相同季度的
//   4) 若匹配不到：取基础名系列里最新（air_date 最大）那条作为系列元数据兜底
//   5) 都不行返回 nil
func (e *Engine) searchBangumi(ctx context.Context, primary string, alts []string) *model.BangumiAnime {
	if e.bangumiSvc == nil || primary == "" {
		return nil
	}
	searchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 构造候选关键词列表（带去重）
	seen := map[string]bool{}
	var queries []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		queries = append(queries, s)
	}
	add(primary)
	for _, a := range alts {
		add(a)
	}
	add(stripSeasonSuffix(primary))
	for _, a := range alts {
		add(stripSeasonSuffix(a))
	}

	wantSeason := extractSeasonFromName(primary)
	if wantSeason == 0 {
		for _, a := range alts {
			if n := extractSeasonFromName(a); n > 0 {
				wantSeason = n
				break
			}
		}
	}

	// 聚合所有候选
	var all []model.BangumiAnime
	hitIDs := map[int]bool{}
	addResults := func(results []model.BangumiAnime) {
		for _, r := range results {
			if hitIDs[r.ID] {
				continue
			}
			hitIDs[r.ID] = true
			all = append(all, r)
		}
	}

	// 第一轮：直接用候选关键词搜
	for _, q := range queries {
		if results, err := e.bangumiSvc.SearchAnime(searchCtx, q); err == nil {
			addResults(results)
		}
	}

	// 第二轮：用第一轮命中条目的"日文原名"再搜（能找全系列续作）
	// 例如第一轮搜"出租女友"只返回 S1+S2；拿 S1 的 Name="彼女、お借りします" 再搜能找到 S1-S5
	if len(all) > 0 {
		nextQueries := map[string]bool{}
		for i, r := range all {
			if i >= 3 {
				break // 前 3 条足够
			}
			jpName := strings.TrimSpace(r.Name)
			// 去掉原名里的"第X季/X期"等后缀（搜系列基础名）
			baseJP := stripSeasonSuffix(jpName)
			if baseJP != "" && !seen[baseJP] && baseJP != jpName {
				nextQueries[baseJP] = true
			}
			if jpName != "" && !seen[jpName] {
				nextQueries[jpName] = true
			}
		}
		for q := range nextQueries {
			if results, err := e.bangumiSvc.SearchAnime(searchCtx, q); err == nil {
				addResults(results)
			}
		}
	}

	if len(all) == 0 {
		zap.L().Debug("RSS 自动发现：Bangumi 搜索无结果",
			zap.String("name", primary), zap.Strings("tried", queries))
		return nil
	}

	// 优先：候选里找匹配 wantSeason 的
	if wantSeason > 0 {
		for _, r := range all {
			s1 := extractSeasonFromName(r.NameCN)
			s2 := extractSeasonFromName(r.Name)
			if s1 == wantSeason || s2 == wantSeason {
				picked := r
				return e.enrichWithDetail(searchCtx, &picked)
			}
		}
	}

	// wantSeason == 1 或 0：取第一条（按 Bangumi 相关度）
	if wantSeason <= 1 {
		picked := all[0]
		return e.enrichWithDetail(searchCtx, &picked)
	}

	// 没匹配到指定季度 —— 取"基础名系列"里 air_date 最新的一条作为兜底（用它的海报/系列信息）
	base := stripSeasonSuffix(primary)
	var fallback *model.BangumiAnime
	var latestDate string
	for i := range all {
		r := &all[i]
		// 相关性：NameCN / Name 含基础名
		if base == "" || strings.Contains(r.NameCN, base) || strings.Contains(r.Name, base) {
			if r.AirDate > latestDate {
				latestDate = r.AirDate
				fallback = r
			}
		}
	}
	if fallback != nil {
		zap.L().Info("RSS 自动发现：未匹配指定季度，用系列最新条目兜底元数据",
			zap.String("name", primary),
			zap.Int("want_season", wantSeason),
			zap.Int("fallback_id", fallback.ID),
			zap.String("fallback_cn", fallback.NameCN))
		return e.enrichWithDetail(searchCtx, fallback)
	}
	return nil
}

// enrichWithDetail 若 search 结果缺 eps_count/air_date，调详情接口补全
func (e *Engine) enrichWithDetail(ctx context.Context, b *model.BangumiAnime) *model.BangumiAnime {
	if b == nil {
		return nil
	}
	if b.EpsCount == 0 || b.AirDate == "" {
		if detail, err := e.bangumiSvc.GetAnimeDetail(ctx, b.ID); err == nil && detail != nil {
			return detail
		}
	}
	return b
}

// enrichFromBangumi 为已存在但元数据不全的 anime 补全。
func (e *Engine) enrichFromBangumi(ctx context.Context, anime *model.Anime) {
	if e.bangumiSvc == nil || anime == nil {
		return
	}
	detail := e.searchBangumi(ctx, anime.Title, nil)
	if detail == nil {
		return
	}
	updates := map[string]interface{}{}
	bangumiID := detail.ID
	if anime.BangumiID == nil {
		updates["bangumi_id"] = bangumiID
	}
	if anime.CoverURL == nil && detail.ImageURL != "" {
		updates["cover_url"] = detail.ImageURL
	}
	if anime.EpisodeCount == nil && detail.EpsCount > 0 {
		updates["episode_count"] = detail.EpsCount
	}
	if anime.BangumiRating == nil && detail.Rating > 0 {
		updates["bangumi_rating"] = detail.Rating
	}
	if anime.Description == nil && detail.Summary != "" {
		updates["description"] = detail.Summary
	}
	if anime.OriginalTitle == nil && detail.Name != "" {
		updates["original_title"] = detail.Name
	}
	if anime.AirWeekday == nil && detail.AirWeekday >= 0 {
		updates["air_weekday"] = detail.AirWeekday
	}
	if anime.Year == nil {
		if y := yearFromAirDate(detail.AirDate); y > 0 {
			updates["year"] = y
		}
	}
	if st := computeStatusFromAirDate(detail.AirDate, detail.EpsCount); st != "" && (anime.Status == "" || anime.Status == model.AnimeStatusUnknown) {
		updates["status"] = st
	}

	if len(updates) > 0 {
		e.db.Model(anime).Updates(updates)
		zap.L().Info("RSS 自动发现：补全番剧元数据",
			zap.String("title", anime.Title),
			zap.Int("fields", len(updates)))
	}
}

// BackfillAutoDiscovered 扫描所有 source_origin=rss_auto 且缺元数据的番剧，统一补全。
// 可通过 HTTP API 手动触发，或定时运行。
func (e *Engine) BackfillAutoDiscovered(ctx context.Context) (int, error) {
	if e.bangumiSvc == nil {
		return 0, nil
	}
	var animes []model.Anime
	err := e.db.WithContext(ctx).
		Where("source_origin = ?", "rss_auto").
		Where("cover_url IS NULL OR episode_count IS NULL").
		Find(&animes).Error
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range animes {
		e.enrichFromBangumi(ctx, &animes[i])
		count++
	}
	zap.L().Info("RSS 自动发现：回填元数据完成", zap.Int("scanned", count))
	return count, nil
}

// yearFromAirDate / computeStatusFromAirDate：复用 anime/service.go 的同名工具
// 在 rss 包中复制一份最小实现避免循环依赖
func yearFromAirDate(airDate string) int {
	if len(airDate) < 4 {
		return 0
	}
	y := 0
	for i := 0; i < 4; i++ {
		c := airDate[i]
		if c < '0' || c > '9' {
			return 0
		}
		y = y*10 + int(c-'0')
	}
	return y
}

func computeStatusFromAirDate(airDate string, epsCount int) string {
	if airDate == "" {
		return model.AnimeStatusUnknown
	}
	t, err := time.Parse("2006-01-02", airDate)
	if err != nil {
		return model.AnimeStatusUnknown
	}
	now := time.Now()
	if t.After(now) {
		return model.AnimeStatusUpcoming
	}
	eps := epsCount
	if eps <= 0 {
		eps = 13
	}
	if t.Add(time.Duration(eps*7) * 24 * time.Hour).Before(now) {
		return model.AnimeStatusFinished
	}
	return model.AnimeStatusOngoing
}

// containsMatch does a case-insensitive substring check.
func containsMatch(title, keyword string) bool {
	return len(keyword) > 0 && searchStr(title, keyword)
}

func searchStr(s, substr string) bool {
	ls := toLower(s)
	lsub := toLower(substr)
	return len(lsub) <= len(ls) && containsLower(ls, lsub)
}

func toLower(s string) string {
	return stringsToLower(s)
}

func stringsToLower(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result = append(result, c)
	}
	return string(result)
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
