package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
)

// BangumiAnime is an alias for model.BangumiAnime.
type BangumiAnime = model.BangumiAnime

// BangumiCalendarDay is an alias for model.BangumiCalendarDay.
type BangumiCalendarDay = model.BangumiCalendarDay

// BangumiService Bangumi API 客户端
type BangumiService struct {
	cfg           *config.Config
	client        *http.Client
	cache         sync.Map
	defaultRules   map[string]*config.XpathRule // 默认规则缓存
	rulesInitOnce  sync.Once
}

var weekdayCN = map[int]string{
	0: "周日", 1: "周一", 2: "周二", 3: "周三",
	4: "周四", 5: "周五", 6: "周六",
}

type cacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

func NewBangumiService(cfg *config.Config) *BangumiService {
	transport := &http.Transport{}
	if cfg.HTTPProxy != "" {
		if proxyURL, err := url.Parse(cfg.HTTPProxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	svc := &BangumiService{
		cfg:    cfg,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		defaultRules: make(map[string]*config.XpathRule),
	}

	// 初始化默认规则
	svc.initDefaultRules()

	return svc
}

// initDefaultRules 初始化默认规则
func (s *BangumiService) initDefaultRules() {
	s.rulesInitOnce.Do(func() {
		// 从配置包加载默认规则
		// 这里我们使用 config 包中定义的默认规则
		for i := range config.DefaultRules {
			rule := &config.DefaultRules[i]
			s.defaultRules[rule.Name] = rule
		}

		zap.L().Info("已加载默认 XPath 规则",
			zap.Int("规则数量", len(s.defaultRules)),
			zap.Strings("规则名称", s.getRuleNames()))
	})
}

// getRuleNames 获取所有规则名称
func (s *BangumiService) getRuleNames() []string {
	names := make([]string, 0, len(s.defaultRules))
	for name := range s.defaultRules {
		names = append(names, name)
	}
	return names
}

func (s *BangumiService) getCached(key string) (interface{}, bool) {
	if val, ok := s.cache.Load(key); ok {
		entry := val.(*cacheEntry)
		if time.Now().Before(entry.ExpiresAt) {
			return entry.Data, true
		}
		s.cache.Delete(key)
	}
	return nil, false
}

func (s *BangumiService) setCache(key string, data interface{}, ttl time.Duration) {
	s.cache.Store(key, &cacheEntry{Data: data, ExpiresAt: time.Now().Add(ttl)})
}

func (s *BangumiService) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	reqURL := strings.TrimRight(s.cfg.BangumiAPIURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "AniDog/1.0 (https://github.com/anidog)")
	if s.cfg.BangumiAccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.BangumiAccessToken)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bangumi API 返回 %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// SearchAnime 搜索番剧
func (s *BangumiService) SearchAnime(ctx context.Context, keyword string) ([]BangumiAnime, error) {
	payload := map[string]interface{}{
		"keyword": keyword,
		"filter": map[string]interface{}{
			"type": []int{2}, // anime
		},
	}
	body, _ := json.Marshal(payload)

	data, err := s.doRequest(ctx, http.MethodPost, "/v0/search/subjects", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			ID         int     `json:"id"`
			Name       string  `json:"name"`
			NameCN     string  `json:"name_cn"`
			Summary    string  `json:"summary"`
			Images     struct {
				Common string `json:"common"`
				Medium string `json:"medium"`
			} `json:"images"`
			Rating struct {
				Score float64 `json:"score"`
			} `json:"rating"`
			AirDate    string `json:"air_date"`
			AirWeekday int    `json:"air_weekday"`
			EpsCount   int    `json:"eps_count"`
			Type       int    `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	var animes []BangumiAnime
	for _, item := range result.Data {
		imageURL := item.Images.Common
		if imageURL == "" {
			imageURL = item.Images.Medium
		}
		animes = append(animes, BangumiAnime{
			ID:         item.ID,
			Name:       item.Name,
			NameCN:     item.NameCN,
			Summary:    item.Summary,
			ImageURL:   imageURL,
			Rating:     item.Rating.Score,
			AirDate:    item.AirDate,
			AirWeekday: item.AirWeekday,
			EpsCount:   item.EpsCount,
			Type:       item.Type,
		})
	}
	return animes, nil
}

// seasonMonthRange 返回某季度的起始/结束月份（用于 Bangumi air_date 过滤）
// 返回 endMonth 为独占上界（>12 表示到次年 1 月）
func seasonMonthRange(season string) (int, int) {
	switch season {
	case "winter":
		return 1, 4
	case "spring":
		return 4, 7
	case "summer":
		return 7, 10
	case "autumn":
		return 10, 13
	}
	return 0, 0
}

// DiscoverOptions 多维度番剧发现参数
type DiscoverOptions struct {
	Keyword   string   // 关键词（可空）
	Sort      string   // "rank" | "score" | "heat" | "match"，默认 "rank"
	Year      int      // 2024 (0=不限)
	Season    string   // 单选，兼容旧字段（空=不限）
	Seasons   []string // 多选，优先使用（空=不限）
	Tags      []string // 标签，如 ["热血","恋爱"]
	MinRating float64  // 最低评分 0-10
	NSFW      bool
	Limit     int
	Offset    int
}

// Discover 按多维度筛选番剧（Bangumi v0 search API）
func (s *BangumiService) Discover(ctx context.Context, opts *DiscoverOptions) ([]BangumiAnime, int, error) {
	filter := map[string]interface{}{
		"type": []int{2},
		"nsfw": opts.NSFW,
	}

	if opts.Year > 0 {
		// 合并 seasons 与 season
		seasons := opts.Seasons
		if len(seasons) == 0 && opts.Season != "" {
			seasons = []string{opts.Season}
		}

		var ranges []string
		if len(seasons) == 0 {
			// 未指定季度 → 整年
			ranges = append(ranges,
				fmt.Sprintf(">=%d-01-01", opts.Year),
				fmt.Sprintf("<%d-01-01", opts.Year+1))
		} else {
			// 多选季度：取所有季度的首播区间并集（Bangumi 的 air_date 过滤支持多段）
			// 简化为取所有选中季度的最小起月 - 最大终月（连续/非连续均可工作，非连续会多召回但可接受）
			minStart, maxEnd := 13, 0
			for _, sv := range seasons {
				start, end := seasonMonthRange(sv)
				if start == 0 {
					continue
				}
				if start < minStart {
					minStart = start
				}
				if end > maxEnd {
					maxEnd = end
				}
			}
			if minStart >= 13 || maxEnd == 0 {
				// fallback 整年
				ranges = append(ranges,
					fmt.Sprintf(">=%d-01-01", opts.Year),
					fmt.Sprintf("<%d-01-01", opts.Year+1))
			} else {
				ranges = append(ranges, fmt.Sprintf(">=%d-%02d-01", opts.Year, minStart))
				if maxEnd > 12 {
					ranges = append(ranges, fmt.Sprintf("<%d-01-01", opts.Year+1))
				} else {
					ranges = append(ranges, fmt.Sprintf("<%d-%02d-01", opts.Year, maxEnd))
				}
			}
		}
		filter["air_date"] = ranges
	}

	if len(opts.Tags) > 0 {
		filter["tag"] = opts.Tags
	}

	if opts.MinRating > 0 {
		filter["rating"] = []string{fmt.Sprintf(">=%.1f", opts.MinRating)}
	}

	sortKey := opts.Sort
	if sortKey == "" {
		sortKey = "rank"
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 24
	}

	payload := map[string]interface{}{
		"keyword": opts.Keyword,
		"sort":    sortKey,
		"filter":  filter,
	}
	body, _ := json.Marshal(payload)

	path := fmt.Sprintf("/v0/search/subjects?limit=%d&offset=%d", limit, opts.Offset)
	data, err := s.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}

	var result struct {
		Total int `json:"total"`
		Data  []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			NameCN  string `json:"name_cn"`
			Summary string `json:"summary"`
			Images  struct {
				Common string `json:"common"`
				Medium string `json:"medium"`
				Large  string `json:"large"`
			} `json:"images"`
			Rating struct {
				Score float64 `json:"score"`
			} `json:"rating"`
			Date     string `json:"date"`
			EpsCount int    `json:"eps"`
			Type     int    `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, err
	}

	var animes []BangumiAnime
	for _, item := range result.Data {
		img := item.Images.Large
		if img == "" {
			img = item.Images.Common
		}
		if img == "" {
			img = item.Images.Medium
		}
		animes = append(animes, BangumiAnime{
			ID:       item.ID,
			Name:     item.Name,
			NameCN:   item.NameCN,
			Summary:  item.Summary,
			ImageURL: img,
			Rating:   item.Rating.Score,
			AirDate:  item.Date,
			EpsCount: item.EpsCount,
			Type:     item.Type,
		})
	}
	return animes, result.Total, nil
}

// GetTrending 获取 Bangumi 热门趋势番剧（参考 Kazumi 首页）
// 数据源：https://next.bgm.tv/p1/trending/subjects?type=2&limit=N&offset=M
func (s *BangumiService) GetTrending(ctx context.Context, limit, offset int) ([]BangumiAnime, int, error) {
	if limit <= 0 {
		limit = 24
	}
	reqURL := fmt.Sprintf("https://next.bgm.tv/p1/trending/subjects?type=2&limit=%d&offset=%d", limit, offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "AniDog/1.0 (Kazumi-compatible)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("trending API 返回 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var result struct {
		Total int `json:"total"`
		Data  []struct {
			Subject struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				NameCN string `json:"nameCN"`
				Type   int    `json:"type"`
				Info   string `json:"info"`
				Rating struct {
					Score float64 `json:"score"`
					Rank  int     `json:"rank"`
				} `json:"rating"`
				Images struct {
					Large  string `json:"large"`
					Common string `json:"common"`
					Medium string `json:"medium"`
				} `json:"images"`
			} `json:"subject"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, err
	}

	var animes []BangumiAnime
	for _, d := range result.Data {
		s := d.Subject
		img := s.Images.Large
		if img == "" {
			img = s.Images.Common
		}
		animes = append(animes, BangumiAnime{
			ID:       s.ID,
			Name:     s.Name,
			NameCN:   s.NameCN,
			ImageURL: img,
			Rating:   s.Rating.Score,
			Rank:     s.Rating.Rank,
			Type:     s.Type,
		})
	}
	return animes, result.Total, nil
}

// GetAnimeDetail 获取番剧详情
func (s *BangumiService) GetAnimeDetail(ctx context.Context, bangumiID int) (*BangumiAnime, error) {
	cacheKey := fmt.Sprintf("detail:%d", bangumiID)
	if cached, ok := s.getCached(cacheKey); ok {
		return cached.(*BangumiAnime), nil
	}

	path := fmt.Sprintf("/v0/subjects/%d", bangumiID)
	data, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var item struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		NameCN  string `json:"name_cn"`
		Summary string `json:"summary"`
		Images  struct {
			Common string `json:"common"`
			Large  string `json:"large"`
		} `json:"images"`
		Rating struct {
			Score float64 `json:"score"`
			Rank  int     `json:"rank"`
		} `json:"rating"`
		Date          string `json:"date"`
		AirDate       string `json:"air_date"`
		AirWeekday    int    `json:"air_weekday"`
		Eps           int    `json:"eps"`
		EpsCount      int    `json:"eps_count"`
		TotalEpisodes int    `json:"total_episodes"`
		Platform      string `json:"platform"`
		Tags          []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Infobox []struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		} `json:"infobox"`
	}
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, err
	}

	airDate := item.AirDate
	if airDate == "" {
		airDate = item.Date
	}
	eps := item.EpsCount
	if eps == 0 {
		eps = item.Eps
	}
	img := item.Images.Large
	if img == "" {
		img = item.Images.Common
	}

	// 解析 infobox（value 可能是 string 或 [{v: ...}]）
	var infobox []model.BangumiInfoKV
	for _, kv := range item.Infobox {
		entry := model.BangumiInfoKV{Key: kv.Key}
		var s string
		if err := json.Unmarshal(kv.Value, &s); err == nil {
			entry.Value = s
		} else {
			var arr []map[string]interface{}
			if err := json.Unmarshal(kv.Value, &arr); err == nil {
				for _, x := range arr {
					if v, ok := x["v"].(string); ok && v != "" {
						entry.Items = append(entry.Items, v)
					}
				}
			}
		}
		infobox = append(infobox, entry)
	}

	var tagNames []string
	for _, t := range item.Tags {
		tagNames = append(tagNames, t.Name)
	}

	anime := &BangumiAnime{
		ID:            item.ID,
		Name:          item.Name,
		NameCN:        item.NameCN,
		Summary:       item.Summary,
		ImageURL:      img,
		Rating:        item.Rating.Score,
		Rank:          item.Rating.Rank,
		AirDate:       airDate,
		AirWeekday:    item.AirWeekday,
		EpsCount:      eps,
		TotalEpisodes: item.TotalEpisodes,
		Platform:      item.Platform,
		Tags:          tagNames,
		Infobox:       infobox,
	}

	s.setCache(cacheKey, anime, 24*time.Hour)
	return anime, nil
}

// GetCharacters 获取番剧角色及声优
func (s *BangumiService) GetCharacters(ctx context.Context, bangumiID int) ([]model.BangumiCharacter, error) {
	cacheKey := fmt.Sprintf("characters:%d", bangumiID)
	if cached, ok := s.getCached(cacheKey); ok {
		return cached.([]model.BangumiCharacter), nil
	}

	path := fmt.Sprintf("/v0/subjects/%d/characters", bangumiID)
	data, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Images struct {
			Medium string `json:"medium"`
			Large  string `json:"large"`
		} `json:"images"`
		Relation string `json:"relation"`
		Actors   []struct {
			Name string `json:"name"`
		} `json:"actors"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var chars []model.BangumiCharacter
	for _, c := range raw {
		actor := ""
		if len(c.Actors) > 0 {
			actor = c.Actors[0].Name
		}
		img := c.Images.Large
		if img == "" {
			img = c.Images.Medium
		}
		chars = append(chars, model.BangumiCharacter{
			ID:       c.ID,
			Name:     c.Name,
			Relation: c.Relation,
			ImageURL: img,
			Actor:    actor,
		})
	}

	s.setCache(cacheKey, chars, 24*time.Hour)
	return chars, nil
}

// GetCharacterDetail 获取角色详情（含描述、infobox、性别等）
func (s *BangumiService) GetCharacterDetail(ctx context.Context, characterID int) (*model.BangumiCharacterDetail, error) {
	cacheKey := fmt.Sprintf("character:%d", characterID)
	if cached, ok := s.getCached(cacheKey); ok {
		return cached.(*model.BangumiCharacterDetail), nil
	}

	path := fmt.Sprintf("/v0/characters/%d", characterID)
	data, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var detail model.BangumiCharacterDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}

	s.setCache(cacheKey, &detail, 24*time.Hour)
	return &detail, nil
}

// BangumiEpisode 单集元信息（来自 Bangumi /v0/episodes）
type BangumiEpisode struct {
	Sort     float64 `json:"sort"`     // 序号（可能是浮点：12.5 = OVA）
	Ep       int     `json:"ep"`       // 集号（正片）
	Name     string  `json:"name"`     // 日文标题
	NameCN   string  `json:"name_cn"`  // 中文标题
	AirDate  string  `json:"airdate"`  // YYYY-MM-DD
	Duration string  `json:"duration"` // "00:23:50"
}

// GetEpisodes 拉取某 anime 的全部正片集（type=0）。
// 返回值按 Bangumi API 原始顺序（通常按 sort 升序）。失败/无数据返回空切片。
//
// 用途：让 anidog 知道每一集的实际播出时间，区分"未下载"和"待发布（air_date
// 在未来）"两种状态——避免 Orchestrator 反复对未播出的集去做无谓搜索，
// 也让前端可以给"还没发布"的格子打上日期标注。
func (s *BangumiService) GetEpisodes(ctx context.Context, bangumiID int) ([]BangumiEpisode, error) {
	cacheKey := fmt.Sprintf("episodes:%d", bangumiID)
	if cached, ok := s.getCached(cacheKey); ok {
		return cached.([]BangumiEpisode), nil
	}

	// type=0 正片，limit=200 一次拉完（一季番不可能超过 200 集）
	path := fmt.Sprintf("/v0/episodes?subject_id=%d&type=0&limit=200", bangumiID)
	data, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Total int              `json:"total"`
		Data  []BangumiEpisode `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析 Bangumi episodes 失败: %w", err)
	}
	// 缓存 1 小时——播出时间表不会频繁变
	s.setCache(cacheKey, resp.Data, time.Hour)
	return resp.Data, nil
}


func (s *BangumiService) GetCalendar(ctx context.Context) ([]BangumiCalendarDay, error) {
	cacheKey := "calendar"
	if cached, ok := s.getCached(cacheKey); ok {
		return cached.([]BangumiCalendarDay), nil
	}

	data, err := s.doRequest(ctx, http.MethodGet, "/calendar", nil)
	if err != nil {
		zap.L().Warn("Bangumi API 获取日历失败，尝试使用默认规则", zap.Error(err))
		return s.GetCalendarWithDefaultRules(ctx)
	}

	var raw []struct {
		Weekday struct {
			ID int `json:"id"`
		} `json:"weekday"`
		Items []struct {
			ID         int     `json:"id"`
			Name       string  `json:"name"`
			NameCN     string  `json:"name_cn"`
			Summary    string  `json:"summary"`
			Images     struct {
				Common string `json:"common"`
			} `json:"images"`
			Rating struct {
				Score float64 `json:"score"`
			} `json:"rating"`
			AirDate    string `json:"air_date"`
			AirWeekday int    `json:"air_weekday"`
			EpsCount   int    `json:"eps_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		zap.L().Warn("Bangumi API 日历解析失败，尝试使用默认规则", zap.Error(err))
		return s.GetCalendarWithDefaultRules(ctx)
	}

	var calendar []BangumiCalendarDay
	for _, day := range raw {
		var items []BangumiAnime
		for _, item := range day.Items {
			items = append(items, BangumiAnime{
				ID:         item.ID,
				Name:       item.Name,
				NameCN:     item.NameCN,
				Summary:    item.Summary,
				ImageURL:   item.Images.Common,
				Rating:     item.Rating.Score,
				AirDate:    item.AirDate,
				AirWeekday: item.AirWeekday,
				EpsCount:   item.EpsCount,
			})
		}
		calendar = append(calendar, BangumiCalendarDay{
			WeekdayID: day.Weekday.ID,
			WeekdayCN: weekdayCN[day.Weekday.ID],
			Items:     items,
		})
	}

	s.setCache(cacheKey, calendar, 6*time.Hour)
	zap.L().Info("Bangumi 日历数据已更新", zap.Int("天数", len(calendar)))
	return calendar, nil
}

// SearchWithDefaultRules 使用默认规则搜索番剧（当 Bangumi API 不可用时）
func (s *BangumiService) SearchWithDefaultRules(ctx context.Context, keyword string) ([]BangumiAnime, error) {
	if len(s.defaultRules) == 0 {
		s.initDefaultRules()
	}

	var allResults []BangumiAnime

	// 使用第一个默认规则进行搜索
	for ruleName, rule := range s.defaultRules {
		selector := config.NewXpathSelector(rule, s.client, ctx)
		results, err := selector.SearchAnime(keyword)
		if err != nil {
			zap.L().Warn("默认规则搜索失败",
				zap.String("规则", ruleName),
				zap.Error(err))
			continue
		}

		// 转换为 BangumiAnime 格式
		for _, result := range results {
			anime := BangumiAnime{
				Name:       s.getString(result, "title"),
				NameCN:     s.getString(result, "name_cn"),
				Summary:    s.getString(result, "summary"),
				ImageURL:   s.getString(result, "image"),
				AirDate:    s.getString(result, "air_date"),
				EpsCount:   s.getInt(result, "eps_count"),
				Rating:     s.getFloat(result, "score"),
				AirWeekday: s.parseWeekday(s.getString(result, "air_weekday")),
			}

			// 从链接生成 ID（使用哈希或简单处理）
			if link, ok := result["link"].(string); ok && link != "" {
				anime.ID = s.generateIDFromLink(link)
			}

			allResults = append(allResults, anime)
		}

		zap.L().Info("默认规则搜索完成",
			zap.String("规则", ruleName),
			zap.Int("结果数", len(results)))

		// 找到结果后不再尝试其他规则
		if len(results) > 0 {
			break
		}
	}

	if len(allResults) == 0 {
		return nil, fmt.Errorf("所有默认规则搜索都失败")
	}

	return allResults, nil
}

// GetDetailWithDefaultRules 使用默认规则获取番剧详情
func (s *BangumiService) GetDetailWithDefaultRules(ctx context.Context, ruleName, detailURL string) (*BangumiAnime, error) {
	rule, ok := s.defaultRules[ruleName]
	if !ok {
		return nil, fmt.Errorf("规则 %s 不存在", ruleName)
	}

	selector := config.NewXpathSelector(rule, s.client, ctx)
	detail, err := selector.GetAnimeDetail(detailURL)
	if err != nil {
		return nil, err
	}

	anime := &BangumiAnime{
		Name:       s.getString(detail, "title"),
		NameCN:     s.getString(detail, "name_cn"),
		Summary:    s.getString(detail, "summary"),
		ImageURL:   s.getString(detail, "image_url"),
		AirDate:    s.getString(detail, "air_date"),
		EpsCount:   s.getInt(detail, "eps_count"),
		Rating:     s.getFloat(detail, "rating"),
		AirWeekday: s.parseWeekday(s.getString(detail, "air_weekday")),
	}

	anime.ID = s.generateIDFromLink(detailURL)

	zap.L().Info("默认规则获取详情完成",
		zap.String("规则", ruleName),
		zap.String("番剧ID", fmt.Sprintf("%d", anime.ID)))

	return anime, nil
}

// SearchAnimeWithFallback 带回退机制的搜索
func (s *BangumiService) SearchAnimeWithFallback(ctx context.Context, keyword string) ([]BangumiAnime, error) {
	// 先尝试使用 Bangumi API
	animes, err := s.SearchAnime(ctx, keyword)
	if err == nil && len(animes) > 0 {
		zap.L().Info("Bangumi API 搜索成功", zap.Int("结果数", len(animes)))
		return animes, nil
	}

	// Bangumi API 失败，检查是否启用默认规则回退
	if !s.cfg.EnableDefaultRules {
		zap.L().Warn("Bangumi API 搜索失败且默认规则回退已禁用",
			zap.Error(err))
		return nil, fmt.Errorf("Bangumi API 搜索失败且默认规则回退已禁用: %w", err)
	}

	zap.L().Warn("Bangumi API 搜索失败，尝试使用默认规则",
		zap.Error(err),
		zap.String("优先规则", s.cfg.DefaultRuleName))

	// 优先使用用户指定的默认规则
	if s.cfg.DefaultRuleName != "" {
		if rule, ok := s.defaultRules[s.cfg.DefaultRuleName]; ok {
			selector := config.NewXpathSelector(rule, s.client, ctx)
			results, err := selector.SearchAnime(keyword)
			if err == nil && len(results) > 0 {
				animes := s.convertResults(results)
				zap.L().Info("默认规则搜索成功",
					zap.String("规则", s.cfg.DefaultRuleName),
					zap.Int("结果数", len(animes)))
				return animes, nil
			}
		}
	}

	// 回退到默认规则
	return s.SearchWithDefaultRules(ctx, keyword)
}

// convertResults 将默认规则的结果转换为 BangumiAnime
func (s *BangumiService) convertResults(results []map[string]interface{}) []BangumiAnime {
	var animes []BangumiAnime
	for _, result := range results {
		anime := BangumiAnime{
			Name:       s.getString(result, "title"),
			NameCN:     s.getString(result, "name_cn"),
			Summary:    s.getString(result, "summary"),
			ImageURL:   s.getString(result, "image"),
			AirDate:    s.getString(result, "air_date"),
			EpsCount:   s.getInt(result, "eps_count"),
			Rating:     s.getFloat(result, "score"),
			AirWeekday: s.parseWeekday(s.getString(result, "air_weekday")),
		}

		// 从链接生成 ID
		if link, ok := result["link"].(string); ok && link != "" {
			anime.ID = s.generateIDFromLink(link)
		}

		animes = append(animes, anime)
	}
	return animes
}

// 辅助方法
func (s *BangumiService) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (s *BangumiService) getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return 0
}

func (s *BangumiService) getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return 0
}

func (s *BangumiService) parseWeekday(weekdayStr string) int {
	weekdayMap := map[string]int{
		"周日": 0, "周一": 1, "周二": 2, "周三": 3, "周四": 4, "周五": 5, "周六": 6,
		"Sun": 0, "Mon": 1, "Tue": 2, "Wed": 3, "Thu": 4, "Fri": 5, "Sat": 6,
		"星期日": 0, "星期一": 1, "星期二": 2, "星期三": 3, "星期四": 4, "星期五": 5, "星期六": 6,
	}

	if weekday, ok := weekdayMap[weekdayStr]; ok {
		return weekday
	}
	return -1
}

func (s *BangumiService) generateIDFromLink(link string) int {
	// 简单的哈希生成 ID
	hash := 0
	for _, c := range link {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// GetCalendarWithDefaultRules 使用默认规则获取日历数据
func (s *BangumiService) GetCalendarWithDefaultRules(ctx context.Context) ([]BangumiCalendarDay, error) {
	if !s.cfg.EnableDefaultRules {
		return nil, fmt.Errorf("Bangumi API 不可用且默认规则已禁用")
	}

	cacheKey := "calendar:default"
	if cached, ok := s.getCached(cacheKey); ok {
		return cached.([]BangumiCalendarDay), nil
	}

	zap.L().Info("使用默认规则获取日历数据",
		zap.String("优先规则", s.cfg.DefaultRuleName))

	// 初始化一周的日历结构
	days := make(map[int]*BangumiCalendarDay)
	for i := 0; i <= 6; i++ {
		days[i] = &BangumiCalendarDay{
			WeekdayID: i,
			WeekdayCN: weekdayCN[i],
			Items:     []BangumiAnime{},
		}
	}

	// 使用默认规则搜索当前热门番剧
	hotKeywords := []string{"海贼王", "进击的巨人", "鬼灭之刃", "咒术回战", "间谍过家家", "电锯人", "蓝色监狱", "我推的孩子", "葬送的芙莉莲"}

	// 优先使用指定的默认规则
	var ruleNames []string
	if s.cfg.DefaultRuleName != "" {
		ruleNames = []string{s.cfg.DefaultRuleName}
	}

	// 如果没有指定规则，尝试所有规则
	if len(ruleNames) == 0 {
		for name := range s.defaultRules {
			ruleNames = append(ruleNames, name)
		}
	}

	// 搜索热门番剧
	for _, keyword := range hotKeywords {
		for _, ruleName := range ruleNames {
			rule, ok := s.defaultRules[ruleName]
			if !ok {
				continue
			}

			selector := config.NewXpathSelector(rule, s.client, ctx)
			results, err := selector.SearchAnime(keyword)
			if err != nil {
				zap.L().Warn("默认规则搜索失败",
					zap.String("规则", ruleName),
					zap.String("关键词", keyword),
					zap.Error(err))
				continue
			}

			// 转换并添加到日历
			for _, result := range results {
				anime := BangumiAnime{
					Name:       s.getString(result, "title"),
					NameCN:     s.getString(result, "name_cn"),
					Summary:    s.getString(result, "summary"),
					ImageURL:   s.getString(result, "image"),
					AirDate:    s.getString(result, "air_date"),
					EpsCount:   s.getInt(result, "eps_count"),
					Rating:     s.getFloat(result, "score"),
					AirWeekday: s.parseWeekday(s.getString(result, "air_weekday")),
				}

				// 从链接生成 ID
				if link, ok := result["link"].(string); ok && link != "" {
					anime.ID = s.generateIDFromLink(link)
				}

				// 如果有放送日期，添加到对应的星期
				if anime.AirWeekday >= 0 && anime.AirWeekday <= 6 {
					days[anime.AirWeekday].Items = append(days[anime.AirWeekday].Items, anime)
				}
			}

			// 找到结果后继续下一个关键词
			if len(results) > 0 {
				break
			}
		}
	}

	// 构建结果数组
	var calendar []BangumiCalendarDay
	for i := 0; i <= 6; i++ {
		if len(days[i].Items) > 0 {
			calendar = append(calendar, *days[i])
		}
	}

	s.setCache(cacheKey, calendar, 24*time.Hour)
	zap.L().Info("默认规则日历数据已生成",
		zap.Int("天数", len(calendar)),
		zap.Int("总番剧数", len(hotKeywords)))

	if len(calendar) == 0 {
		return nil, fmt.Errorf("无法从默认规则获取日历数据")
	}

	return calendar, nil
}
