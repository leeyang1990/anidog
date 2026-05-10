package indexer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// DmhyIndexer 动漫花园
// GET https://www.dmhy.org/topics/list/page/1?keyword=KEYWORD&sort_id=2 (sort_id=2 = 動畫)
type DmhyIndexer struct {
	BaseURL string
	Client  *http.Client
}

func NewDmhyIndexer() *DmhyIndexer {
	return &DmhyIndexer{
		BaseURL: "https://www.dmhy.org",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DmhyIndexer) Name() string { return "dmhy" }

func (d *DmhyIndexer) Search(ctx context.Context, keyword string) ([]Candidate, error) {
	if d.BaseURL == "" {
		d.BaseURL = "https://www.dmhy.org"
	}
	if d.Client == nil {
		d.Client = &http.Client{Timeout: 15 * time.Second}
	}

	u := d.BaseURL + "/topics/list/page/1?keyword=" + url.QueryEscape(keyword) + "&sort_id=2"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anidog/1.0)")

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dmhy 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dmhy 返回状态码 %d", resp.StatusCode)
	}

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dmhy 解析 HTML 失败: %w", err)
	}

	// 仅选 tbody 内的 <tr>，过滤表头
	rows := htmlquery.Find(doc, `//table[@id="topic_list"]/tbody/tr`)
	out := make([]Candidate, 0, len(rows))
	for _, row := range rows {
		if c := parseDmhyRow(row, d.BaseURL); c.Title != "" {
			out = append(out, c)
		}
	}
	return out, nil
}

func parseDmhyRow(row *html.Node, baseURL string) Candidate {
	c := Candidate{SourceName: "dmhy"}

	tds := htmlquery.Find(row, `./td`)
	if len(tds) < 5 {
		return c
	}

	// td[0]: 时间 "2026/05/02 00:24"
	dateStr := strings.TrimSpace(htmlquery.InnerText(tds[0]))
	// 清理可能的重复内容（dmhy 这里有隐藏 span 同样内容）
	if idx := strings.Index(dateStr, "\n"); idx > 0 {
		dateStr = strings.TrimSpace(dateStr[:idx])
	}
	dateStr = strings.Join(strings.Fields(dateStr), " ")
	// 取前 16 字符尝试解析 "YYYY/MM/DD HH:MM"
	if len(dateStr) >= 16 {
		if t, err := time.Parse("2006/01/02 15:04", dateStr[:16]); err == nil {
			c.PubDate = t
		}
	}

	// td[2]: title.  里面包括可选的 [字幕组] tag + <a href="/topics/view/..." >文案</a>
	titleTd := tds[2]
	if titleA := htmlquery.FindOne(titleTd, `./a[contains(@href,"/topics/view/")]`); titleA != nil {
		// 清理掉 <span class="keyword"> 高亮的情况，直接取 InnerText 即可（keyword 段落也是文本）
		c.Title = cleanSpaces(htmlquery.InnerText(titleA))
		if href := htmlquery.SelectAttr(titleA, "href"); href != "" {
			c.DetailURL = baseURL + href
		}
	}

	// td[3]: 磁力
	if magA := htmlquery.FindOne(tds[3], `./a[contains(@class,"arrow-magnet")]`); magA != nil {
		if href := htmlquery.SelectAttr(magA, "href"); href != "" {
			c.MagnetURL = href
			if m := reInfoHash.FindStringSubmatch(href); m != nil {
				c.InfoHash = strings.ToUpper(m[1])
			}
		}
	}

	// td[4]: 体积 "809.9MB"
	c.Size = parseHumanSize(strings.TrimSpace(htmlquery.InnerText(tds[4])))

	// 可选: td[5/6] 做种/下载
	if len(tds) >= 7 {
		c.Seeders = parseIntSafe(strings.TrimSpace(htmlquery.InnerText(tds[5])))
		c.Leechers = parseIntSafe(strings.TrimSpace(htmlquery.InnerText(tds[6])))
	}

	return c
}

func cleanSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func parseIntSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}
