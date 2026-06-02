package indexer

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// MikanRSSFetcher 通过 Mikan 的番剧 RSS 拉候选种子。
//
// 与 MikanIndexer（关键词搜索）不同，RSS 路线：
//   - 输入是 mikan_bangumi_id（订阅时反查存在 anime 表里）
//   - 一次性返回该番所有字幕组的所有集（典型 30-300 条）
//   - InfoHash 直接在 link URL 末段，无需爬详情页
//   - 不暴露 seeders（设 SeedersReported=false 避免误杀）
//
// URL: https://mikanani.me/RSS/Bangumi?bangumiId=X
type MikanRSSFetcher struct {
	BaseURL string
	Client  *http.Client
}

func NewMikanRSSFetcher() *MikanRSSFetcher {
	return &MikanRSSFetcher{
		BaseURL: "https://mikanani.me",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Fetch 拉取该 mikan_bangumi_id 的全部 RSS items。
func (f *MikanRSSFetcher) Fetch(ctx context.Context, mikanBangumiID int) ([]Candidate, error) {
	if mikanBangumiID <= 0 {
		return nil, fmt.Errorf("mikan_bangumi_id 无效: %d", mikanBangumiID)
	}
	if f.BaseURL == "" {
		f.BaseURL = "https://mikanani.me"
	}
	if f.Client == nil {
		f.Client = &http.Client{Timeout: 15 * time.Second}
	}

	url := fmt.Sprintf("%s/RSS/Bangumi?bangumiId=%d", f.BaseURL, mikanBangumiID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anidog/1.0)")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mikan rss 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mikan rss HTTP %d", resp.StatusCode)
	}

	var feed mikanRSSFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("mikan rss 解析失败: %w", err)
	}

	out := make([]Candidate, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		c := Candidate{
			Title:           strings.TrimSpace(it.Title),
			TorrentURL:      strings.TrimSpace(it.Enclosure.URL),
			Size:            it.Enclosure.Length,
			SourceName:      "mikan_rss",
			DetailURL:       strings.TrimSpace(it.Link),
			SeedersReported: false, // Mikan RSS 不暴露种子数
		}
		// link 末段是 InfoHash: /Home/Episode/<40 hex>
		if h := extractInfoHashFromMikanLink(it.Link); h != "" {
			c.InfoHash = h
			// 没 magnet 时拼一个（让下游可以走 magnet 流程）
			c.MagnetURL = "magnet:?xt=urn:btih:" + h
		}
		// pubDate
		if it.Torrent.PubDate != "" {
			if t, err := time.Parse("2006-01-02T15:04:05.999999", it.Torrent.PubDate); err == nil {
				c.PubDate = t
			} else if t, err := time.Parse("2006-01-02T15:04:05", it.Torrent.PubDate); err == nil {
				c.PubDate = t
			}
		}

		if c.Title == "" || c.InfoHash == "" {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

type mikanRSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []mikanRSSItem `xml:"item"`
	} `xml:"channel"`
}

type mikanRSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Torrent     struct {
		Link          string `xml:"link"`
		ContentLength int64  `xml:"contentLength"`
		PubDate       string `xml:"pubDate"`
	} `xml:"torrent"`
	Enclosure struct {
		URL    string `xml:"url,attr"`
		Length int64  `xml:"length,attr"`
		Type   string `xml:"type,attr"`
	} `xml:"enclosure"`
}

var reMikanLinkHash = regexp.MustCompile(`/Home/Episode/([a-fA-F0-9]{40})`)

func extractInfoHashFromMikanLink(link string) string {
	m := reMikanLinkHash.FindStringSubmatch(link)
	if m == nil {
		return ""
	}
	return strings.ToUpper(m[1])
}
