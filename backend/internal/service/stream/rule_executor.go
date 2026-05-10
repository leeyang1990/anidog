package stream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/network"
)

// SearchResult 搜索结果
type SearchResult struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	CoverURL string `json:"cover_url,omitempty"`
	RuleName string `json:"rule_name,omitempty"`
}

// EpisodeInfo 集数信息
type EpisodeInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	RoadName string `json:"road_name,omitempty"`
}

// BrowserProvider 提供共享的 rod 浏览器实例。nil 表示当前不可用。
type BrowserProvider interface {
	GetBrowser() *rod.Browser
}

// StreamRuleExecutor XPath 规则执行器
type StreamRuleExecutor struct {
	httpClient *network.HTTPClient
	browser    BrowserProvider
}

func NewStreamRuleExecutor(httpClient *network.HTTPClient, browser BrowserProvider) *StreamRuleExecutor {
	return &StreamRuleExecutor{httpClient: httpClient, browser: browser}
}

// needsBrowser 判断规则是否需要浏览器渲染。
func needsBrowser(rule *model.StreamRule) bool {
	if rule.UseWebview {
		return true
	}
	if rule.AntiCrawlerConfig != nil && *rule.AntiCrawlerConfig != "" && !strings.Contains(*rule.AntiCrawlerConfig, `"enabled":false`) {
		return true
	}
	return false
}

// Search 使用规则搜索番剧
func (e *StreamRuleExecutor) Search(ctx context.Context, rule *model.StreamRule, keyword string) ([]SearchResult, error) {
	searchURL := strings.ReplaceAll(rule.SearchURL, "@keyword", url.PathEscape(keyword))

	var htmlStr string
	var err error

	if needsBrowser(rule) && e.browser != nil && e.browser.GetBrowser() != nil {
		htmlStr, err = e.fetchWithBrowser(ctx, rule, searchURL, keyword, true)
	} else {
		htmlStr, err = e.fetchWithHTTP(ctx, rule, searchURL, keyword)
	}
	if err != nil {
		return nil, err
	}

	doc, err := htmlquery.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	listNodes := htmlquery.Find(doc, rule.SearchListXPath)
	var results []SearchResult

	for _, node := range listNodes {
		nameNode := htmlquery.FindOne(node, rule.SearchNameXPath)
		resultNode := htmlquery.FindOne(node, rule.SearchResultXPath)

		if nameNode == nil || resultNode == nil {
			continue
		}

		name := strings.TrimSpace(htmlquery.InnerText(nameNode))
		href := htmlquery.SelectAttr(resultNode, "href")
		if href == "" {
			href = strings.TrimSpace(htmlquery.InnerText(resultNode))
		}

		if name == "" || href == "" {
			continue
		}

		fullURL := e.resolveURL(rule.BaseURL, href)
		results = append(results, SearchResult{
			Name:     name,
			URL:      fullURL,
			RuleName: rule.Name,
		})
	}

	zap.L().Debug("流媒体搜索完成", zap.String("rule", rule.Name), zap.String("keyword", keyword), zap.Int("结果数", len(results)))
	return results, nil
}

// ParseEpisodes 解析番剧详情页的集数列表
// 按照 Kazumi 的 querychapterRoads 逻辑实现：
//   - 有 chapterRoadsXPath 时：XPath 匹配多个线路容器，每个容器内用 chapterResultXPath 提取剧集
//   - 无 chapterRoadsXPath 时：单线路模式，直接用 chapterResultXPath 提取所有剧集
func (e *StreamRuleExecutor) ParseEpisodes(ctx context.Context, rule *model.StreamRule, detailURL string) ([]EpisodeInfo, error) {
	var htmlStr string
	var err error

	if needsBrowser(rule) && e.browser != nil && e.browser.GetBrowser() != nil {
		htmlStr, err = e.fetchWithBrowser(ctx, rule, detailURL, "", false)
	} else {
		htmlStr, err = e.fetchWithHTTP(ctx, rule, detailURL, "")
	}
	if err != nil {
		return nil, err
	}

	doc, err := htmlquery.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	var episodes []EpisodeInfo

	if rule.ChapterRoadsXPath != nil && *rule.ChapterRoadsXPath != "" {
		// 多线路模式（与 Kazumi querychapterRoads 一致）
		// chapterRoadsXPath 匹配到的节点数量 = 播放线路数量
		roadNodes := htmlquery.Find(doc, *rule.ChapterRoadsXPath)
		zap.L().Info("多线路解析", zap.String("rule", rule.Name), zap.Int("线路数", len(roadNodes)))

		for i, roadNode := range roadNodes {
			// 线路自动编号：播放列表1, 播放列表2, ...
			roadName := fmt.Sprintf("播放列表%d", i+1)

			// 在每个线路容器内，用 chapterResultXPath 提取剧集链接
			epNodes := htmlquery.Find(roadNode, rule.ChapterResultXPath)
			for _, epNode := range epNodes {
				name := strings.TrimSpace(htmlquery.InnerText(epNode))
				// Kazumi: itemName.replaceAll(RegExp(r'\s+'), '')
				name = strings.ReplaceAll(name, " ", "")
				href := htmlquery.SelectAttr(epNode, "href")
				if name == "" || href == "" {
					continue
				}
				episodes = append(episodes, EpisodeInfo{
					Name:     name,
					URL:      e.resolveURL(rule.BaseURL, href),
					RoadName: roadName,
				})
			}
		}
	} else {
		// 单线路模式
		epNodes := htmlquery.Find(doc, rule.ChapterResultXPath)
		for _, epNode := range epNodes {
			name := strings.TrimSpace(htmlquery.InnerText(epNode))
			href := htmlquery.SelectAttr(epNode, "href")
			if name == "" || href == "" {
				continue
			}
			episodes = append(episodes, EpisodeInfo{
				Name: name,
				URL:  e.resolveURL(rule.BaseURL, href),
			})
		}
	}

	zap.L().Info("集数解析完成", zap.String("url", detailURL), zap.Int("集数", len(episodes)))
	return episodes, nil
}

// fetchWithHTTP 直接 HTTP 请求。
func (e *StreamRuleExecutor) fetchWithHTTP(ctx context.Context, rule *model.StreamRule, pageURL, keyword string) (string, error) {
	var body io.Reader
	method := http.MethodGet
	if rule.UsePost && keyword != "" {
		method = http.MethodPost
		form := url.Values{"keyword": {keyword}, "wd": {keyword}}
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, pageURL, body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	e.setHeaders(req, rule)

	resp, err := e.httpClient.Client().Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}
	return string(data), nil
}

// fetchWithBrowser 用 rod 浏览器加载页面，等待 JS 渲染后返回 HTML。
func (e *StreamRuleExecutor) fetchWithBrowser(ctx context.Context, rule *model.StreamRule, pageURL, keyword string, isSearch bool) (string, error) {
	browser := e.browser.GetBrowser()
	if browser == nil {
		return "", fmt.Errorf("浏览器不可用")
	}

	opCtx, opCancel := context.WithTimeout(ctx, 30*time.Second)
	defer opCancel()

	page, err := browser.Context(opCtx).Page(proto.TargetCreateTarget{})
	if err != nil {
		return "", fmt.Errorf("创建页面失败: %w", err)
	}
	defer page.Close()

	_, _ = page.EvalOnNewDocument(`() => {
		Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
		Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });
		Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
		window.chrome = { runtime: {} };
	}`)

	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	if rule.UserAgent != nil && *rule.UserAgent != "" {
		ua = *rule.UserAgent
	}
	_ = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent:      ua,
		AcceptLanguage: "zh-CN,zh;q=0.9,en;q=0.8",
		Platform:       "Win32",
	})

	_ = proto.NetworkSetExtraHTTPHeaders{
		Headers: proto.NetworkHeaders{
			"Accept":             gson.New("text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"),
			"Accept-Language":    gson.New("zh-CN,zh;q=0.9,en;q=0.8"),
			"sec-ch-ua":          gson.New(`"Chromium";v="126", "Google Chrome";v="126", "Not=A?Brand";v="24"`),
			"sec-ch-ua-mobile":   gson.New("?0"),
			"sec-ch-ua-platform": gson.New(`"Windows"`),
		},
	}.Call(page)

	if err := page.Context(opCtx).Navigate(pageURL); err != nil {
		return "", fmt.Errorf("导航失败: %w", err)
	}

	_ = page.Context(opCtx).WaitLoad()

	select {
	case <-time.After(1500 * time.Millisecond):
	case <-opCtx.Done():
	}

	html, err := page.HTML()
	if err != nil {
		return "", fmt.Errorf("提取 HTML 失败: %w", err)
	}

	_ = isSearch
	return html, nil
}

func (e *StreamRuleExecutor) setHeaders(req *http.Request, rule *model.StreamRule) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if rule.UserAgent != nil && *rule.UserAgent != "" {
		req.Header.Set("User-Agent", *rule.UserAgent)
	}
	if rule.Referer != nil && *rule.Referer != "" {
		req.Header.Set("Referer", *rule.Referer)
	} else {
		req.Header.Set("Referer", rule.BaseURL)
	}
}

func (e *StreamRuleExecutor) resolveURL(baseURL, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}

	ref, err := url.Parse(href)
	if err != nil {
		return href
	}

	return base.ResolveReference(ref).String()
}
