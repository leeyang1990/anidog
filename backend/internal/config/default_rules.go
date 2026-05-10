package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

// XpathRule XPath 规则配置
type XpathRule struct {
	Name        string            `json:"name"`        // 规则名称
	Description string            `json:"description"` // 规则描述
	BaseURL     string            `json:"base_url"`    // 基础 URL
	Encoding    string            `json:"encoding"`    // 页面编码

	// 搜索规则
	SearchPath     string `json:"search_path"`     // 搜索结果路径
	SearchTitle    string `json:"search_title"`    // 标题选择器
	SearchLink     string `json:"search_link"`     // 链接选择器
	SearchImage    string `json:"search_image"`    // 图片选择器
	SearchScore    string `json:"search_score"`    // 评分选择器
	SearchAirDate  string `json:"search_air_date"` // 首播日期选择器
	SearchEpsCount string `json:"search_eps_count"` // 集数选择器

	// 详情规则
	DetailTitle     string `json:"detail_title"`      // 标题选择器
	DetailNameCN    string `json:"detail_name_cn"`    // 中文名选择器
	DetailSummary   string `json:"detail_summary"`    // 摘要选择器
	DetailImage     string `json:"detail_image"`     // 封面图选择器
	DetailScore     string `json:"detail_score"`      // 评分选择器
	DetailAirDate   string `json:"detail_air_date"`  // 首播日期选择器
	DetailEpsCount  string `json:"detail_eps_count"`  // 集数选择器
	DetailAirWeekday string `json:"detail_air_weekday"` // 首播星期选择器
	DetailType      string `json:"detail_type"`       // 类型选择器
}

// DefaultRules 默认规则集合 - 参考 Kazumi 项目的实现
var DefaultRules = []XpathRule{
	{
		Name:        "mikan",
		Description: "Mikan Project - 主要动漫资源站点",
		BaseURL:     "https://mikanani.me",
		Encoding:    "utf-8",

		// 搜索规则
		SearchPath:     "/Home/Search?searchstr=",
		SearchTitle:    ".torrent-title",
		SearchLink:     "a",
		SearchImage:    ".torrent-poster img",
		SearchScore:    ".rating",
		SearchAirDate:  ".air-date",
		SearchEpsCount: ".eps-count",

		// 详情规则
		DetailTitle:       ".torrent-title",
		DetailNameCN:      ".torrent-sub-title",
		DetailSummary:     ".description",
		DetailImage:       ".torrent-poster img",
		DetailScore:       ".rating",
		DetailAirDate:     ".air-date",
		DetailEpsCount:    ".eps-count",
		DetailAirWeekday:  ".weekday",
		DetailType:        ".type",
	},
	{
		Name:        "dmhy",
		Description: "动漫花园 - 传统动漫资源站",
		BaseURL:     "https://share.dmhy.org",
		Encoding:    "utf-8",

		// 搜索规则
		SearchPath:     "/topics/list?keyword=",
		SearchTitle:    "a",
		SearchLink:     "a",
		SearchImage:    "img",
		SearchScore:    ".score",
		SearchAirDate:  ".date",
		SearchEpsCount: ".eps",

		// 详情规则
		DetailTitle:       "h1.title",
		DetailNameCN:      ".cn-name",
		DetailSummary:     ".summary",
		DetailImage:       ".cover img",
		DetailScore:       ".rating .score",
		DetailAirDate:     ".info .air-date",
		DetailEpsCount:    ".info .eps-count",
		DetailAirWeekday:  ".info .weekday",
		DetailType:        ".info .type",
	},
	{
		Name:        "bangumi_no_login",
		Description: "Bangumi 番组计划 - 无登录模式",
		BaseURL:     "https://bgm.tv",
		Encoding:    "utf-8",

		// 搜索规则 - 直接搜索页面
		SearchPath:     "/subject_search/",
		SearchTitle:    "h3 a",
		SearchLink:     "h3 a",
		SearchImage:    "img",
		SearchScore:    ".rateInfo",
		SearchAirDate:  ".info .pubDate",
		SearchEpsCount: ".info .epsCount",

		// 详情规则
		DetailTitle:       "h1.nameSingle",
		DetailNameCN:      "h1.nameSingle a",
		DetailSummary:     "#subject_summary",
		DetailImage:       "img.cover",
		DetailScore:       ".global_score .number",
		DetailAirDate:     ".info .pubDate",
		DetailEpsCount:    ".info .epsCount",
		DetailAirWeekday:  ".info .onAir",
		DetailType:        ".badge",
	},
	{
		Name:        "bilibili_anime",
		Description: "哔哩哔哩番剧页面",
		BaseURL:     "https://www.bilibili.com",
		Encoding:    "utf-8",

		// 搜索规则
		SearchPath:     "/search?keyword=",
		SearchTitle:    "a.title",
		SearchLink:     "a.title",
		SearchImage:    "img",
		SearchScore:    ".score",
		SearchAirDate:  ".date",
		SearchEpsCount: ".eps",

		// 详情规则
		DetailTitle:       "h1",
		DetailNameCN:      "h1",
		DetailSummary:     ".info",
		DetailImage:       "img.cover",
		DetailScore:       ".rating",
		DetailAirDate:     ".pubdate",
		DetailEpsCount:    ".epcount",
		DetailAirWeekday:  ".weekday",
		DetailType:        ".type",
	},
	{
		Name:        "acg_rice",
		Description: "ACG.RIP 番剧资源站",
		BaseURL:     "https://acg.rip",
		Encoding:    "utf-8",

		// 搜索规则
		SearchPath:     "/?term=",
		SearchTitle:    "a",
		SearchLink:     "a",
		SearchImage:    "img",
		SearchScore:    ".score",
		SearchAirDate:  ".date",
		SearchEpsCount: ".eps",

		// 详情规则
		DetailTitle:       "h1",
		DetailNameCN:      "h1",
		DetailSummary:     ".description",
		DetailImage:       "img.cover",
		DetailScore:       ".rating",
		DetailAirDate:     ".date",
		DetailEpsCount:    ".eps-count",
		DetailAirWeekday:  ".weekday",
		DetailType:        ".type",
	},
}

// GetRuleByName 根据规则名称获取规则
func GetRuleByName(name string) (*XpathRule, bool) {
	for _, rule := range DefaultRules {
		if rule.Name == name {
			return &rule, true
		}
	}
	return nil, false
}

// XpathSelector XPath 选择器解析器
type XpathSelector struct {
	rule    *XpathRule
	client  *http.Client
	context context.Context
}

// NewXpathSelector 创建 XPath 选择器
func NewXpathSelector(rule *XpathRule, client *http.Client, ctx context.Context) *XpathSelector {
	return &XpathSelector{
		rule:    rule,
		client:  client,
		context: ctx,
	}
}

// SearchAnime 使用 XPath 规则搜索番剧
func (xs *XpathSelector) SearchAnime(keyword string) ([]map[string]interface{}, error) {
	if xs.rule.SearchPath == "" {
		return nil, fmt.Errorf("搜索规则未配置")
	}

	url := xs.rule.BaseURL + xs.rule.SearchPath + url.QueryEscape(keyword)

	zap.L().Info("使用 XPath 规则搜索番剧",
		zap.String("rule", xs.rule.Name),
		zap.String("url", url))

	req, err := http.NewRequestWithContext(xs.context, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if xs.rule.Encoding == "" {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}

	resp, err := xs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	var results []map[string]interface{}

	doc.Find(xs.rule.SearchPath).Each(func(i int, s *goquery.Selection) {
		result := make(map[string]interface{})

		// 提取标题
		if xs.rule.SearchTitle != "" {
			if title := s.Find(xs.rule.SearchTitle).First().Text(); title != "" {
				result["title"] = strings.TrimSpace(title)
			}
		}

		// 提取链接
		if xs.rule.SearchLink != "" {
			if link, ok := s.Find(xs.rule.SearchLink).First().Attr("href"); ok {
				if !strings.HasPrefix(link, "http") {
					link = xs.rule.BaseURL + link
				}
				result["link"] = link
			}
		}

		// 提取图片
		if xs.rule.SearchImage != "" {
			if img, ok := s.Find(xs.rule.SearchImage).First().Attr("src"); ok {
				if !strings.HasPrefix(img, "http") {
					img = xs.rule.BaseURL + img
				}
				result["image"] = img
			}
		}

		// 提取评分
		if xs.rule.SearchScore != "" {
			if score := s.Find(xs.rule.SearchScore).First().Text(); score != "" {
				result["score"] = strings.TrimSpace(score)
			}
		}

		// 提取首播日期
		if xs.rule.SearchAirDate != "" {
			if airDate := s.Find(xs.rule.SearchAirDate).First().Text(); airDate != "" {
				result["air_date"] = strings.TrimSpace(airDate)
			}
		}

		// 提取集数
		if xs.rule.SearchEpsCount != "" {
			if epsCount := s.Find(xs.rule.SearchEpsCount).First().Text(); epsCount != "" {
				result["eps_count"] = strings.TrimSpace(epsCount)
			}
		}

		result["source"] = xs.rule.Name
		results = append(results, result)
	})

	zap.L().Info("XPath 搜索完成",
		zap.String("rule", xs.rule.Name),
		zap.Int("results", len(results)))

	return results, nil
}

// GetAnimeDetail 获取番剧详情
func (xs *XpathSelector) GetAnimeDetail(detailURL string) (map[string]interface{}, error) {
	if detailURL == "" {
		return nil, fmt.Errorf("详情 URL 为空")
	}

	// 如果是相对 URL，拼接基础 URL
	if !strings.HasPrefix(detailURL, "http") {
		detailURL = xs.rule.BaseURL + detailURL
	}

	zap.L().Info("获取番剧详情",
		zap.String("rule", xs.rule.Name),
		zap.String("url", detailURL))

	req, err := http.NewRequestWithContext(xs.context, http.MethodGet, detailURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := xs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	detail := make(map[string]interface{})

	// 提取标题
	if xs.rule.DetailTitle != "" {
		if title := doc.Find(xs.rule.DetailTitle).First().Text(); title != "" {
			detail["title"] = strings.TrimSpace(title)
		}
	}

	// 提取中文名
	if xs.rule.DetailNameCN != "" {
		if nameCN := doc.Find(xs.rule.DetailNameCN).First().Text(); nameCN != "" {
			detail["name_cn"] = strings.TrimSpace(nameCN)
		}
	}

	// 提取摘要
	if xs.rule.DetailSummary != "" {
		if summary := doc.Find(xs.rule.DetailSummary).First().Text(); summary != "" {
			detail["summary"] = strings.TrimSpace(summary)
		}
	}

	// 提取封面图
	if xs.rule.DetailImage != "" {
		if img, ok := doc.Find(xs.rule.DetailImage).First().Attr("src"); ok {
			if !strings.HasPrefix(img, "http") {
				img = xs.rule.BaseURL + img
			}
			detail["image_url"] = img
		}
	}

	// 提取评分
	if xs.rule.DetailScore != "" {
		if score := doc.Find(xs.rule.DetailScore).First().Text(); score != "" {
			detail["rating"] = strings.TrimSpace(score)
		}
	}

	// 提取首播日期
	if xs.rule.DetailAirDate != "" {
		if airDate := doc.Find(xs.rule.DetailAirDate).First().Text(); airDate != "" {
			detail["air_date"] = strings.TrimSpace(airDate)
		}
	}

	// 提取集数
	if xs.rule.DetailEpsCount != "" {
		if epsCount := doc.Find(xs.rule.DetailEpsCount).First().Text(); epsCount != "" {
			detail["eps_count"] = strings.TrimSpace(epsCount)
		}
	}

	// 提取首播星期
	if xs.rule.DetailAirWeekday != "" {
		if weekday := doc.Find(xs.rule.DetailAirWeekday).First().Text(); weekday != "" {
			detail["air_weekday"] = strings.TrimSpace(weekday)
		}
	}

	// 提取类型
	if xs.rule.DetailType != "" {
		if typeInfo := doc.Find(xs.rule.DetailType).First().Text(); typeInfo != "" {
			detail["type"] = strings.TrimSpace(typeInfo)
		}
	}

	detail["source"] = xs.rule.Name

	return detail, nil
}

// SearchWithTimeout 带超时的搜索
func SearchWithTimeout(ruleName, keyword string, timeout time.Duration) ([]map[string]interface{}, error) {
	rule, ok := GetRuleByName(ruleName)
	if !ok {
		return nil, fmt.Errorf("规则 %s 不存在", ruleName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{
		Timeout: timeout,
	}

	selector := NewXpathSelector(rule, client, ctx)
	return selector.SearchAnime(keyword)
}

// GetDetailWithTimeout 带超时的详情获取
func GetDetailWithTimeout(ruleName, detailURL string, timeout time.Duration) (map[string]interface{}, error) {
	rule, ok := GetRuleByName(ruleName)
	if !ok {
		return nil, fmt.Errorf("规则 %s 不存在", ruleName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{
		Timeout: timeout,
	}

	selector := NewXpathSelector(rule, client, ctx)
	return selector.GetAnimeDetail(detailURL)
}
