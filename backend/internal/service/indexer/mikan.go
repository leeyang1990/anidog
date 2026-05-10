package indexer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// MikanIndexer 蜜柑计划搜索实现
// GET https://mikanani.me/Home/Search?searchstr=KEYWORD
type MikanIndexer struct {
	BaseURL string
	Client  *http.Client
}

func NewMikanIndexer() *MikanIndexer {
	return &MikanIndexer{
		BaseURL: "https://mikanani.me",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MikanIndexer) Name() string { return "mikan" }

func (m *MikanIndexer) Search(ctx context.Context, keyword string) ([]Candidate, error) {
	if m.BaseURL == "" {
		m.BaseURL = "https://mikanani.me"
	}
	if m.Client == nil {
		m.Client = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		m.BaseURL+"/Home/Search?searchstr="+url.QueryEscape(keyword), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anidog/1.0)")

	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mikan 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mikan 返回状态码 %d", resp.StatusCode)
	}

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mikan 解析 HTML 失败: %w", err)
	}

	rows := htmlquery.Find(doc, `//tr[contains(@class,"js-search-results-row")]`)
	out := make([]Candidate, 0, len(rows))
	for _, row := range rows {
		if c := parseMikanRow(row, m.BaseURL); c.Title != "" {
			out = append(out, c)
		}
	}
	return out, nil
}

var reInfoHash = regexp.MustCompile(`btih:([a-fA-F0-9]{40})`)

func parseMikanRow(row *html.Node, baseURL string) Candidate {
	c := Candidate{SourceName: "mikan"}

	// 标题 + 详情
	if titleA := htmlquery.FindOne(row, `.//a[contains(@class,"magnet-link-wrap")]`); titleA != nil {
		c.Title = strings.TrimSpace(htmlquery.InnerText(titleA))
		if href := htmlquery.SelectAttr(titleA, "href"); href != "" {
			c.DetailURL = baseURL + href
		}
	}

	// 磁力
	if magnetA := htmlquery.FindOne(row, `.//a[contains(@class,"js-magnet")]`); magnetA != nil {
		if mag := htmlquery.SelectAttr(magnetA, "data-clipboard-text"); mag != "" {
			c.MagnetURL = mag
			if m := reInfoHash.FindStringSubmatch(mag); m != nil {
				c.InfoHash = strings.ToUpper(m[1])
			}
		}
	}

	// .torrent 直链
	if dl := htmlquery.FindOne(row, `.//a[contains(@href,"/Download/")]`); dl != nil {
		if href := htmlquery.SelectAttr(dl, "href"); href != "" {
			c.TorrentURL = baseURL + href
		}
	}

	// 3/4 列是体积、日期
	tds := htmlquery.Find(row, `./td`)
	if len(tds) >= 4 {
		c.Size = parseHumanSize(strings.TrimSpace(htmlquery.InnerText(tds[2])))
		dateStr := strings.TrimSpace(htmlquery.InnerText(tds[3]))
		if t, err := time.Parse("2006/01/02 15:04", dateStr); err == nil {
			c.PubDate = t
		}
	}

	return c
}
