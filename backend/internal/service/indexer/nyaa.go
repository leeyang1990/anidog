package indexer

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NyaaIndexer nyaa.si 英语/日语圈主流
// GET https://nyaa.si/?page=rss&q=KEYWORD
type NyaaIndexer struct {
	BaseURL string
	Client  *http.Client
}

func NewNyaaIndexer() *NyaaIndexer {
	return &NyaaIndexer{
		BaseURL: "https://nyaa.si",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (n *NyaaIndexer) Name() string { return "nyaa" }

// nyaaRSS 采用手动 XML 解析，因为 gofeed 不直接暴露 nyaa:* 扩展字段
type nyaaRSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel nyaaChan   `xml:"channel"`
}
type nyaaChan struct {
	Items []nyaaItem `xml:"item"`
}
type nyaaItem struct {
	Title    string `xml:"title"`
	Link     string `xml:"link"`
	GUID     string `xml:"guid"`
	PubDate  string `xml:"pubDate"`
	Seeders  string `xml:"http://nyaa.si/ns seeders"`
	Leechers string `xml:"http://nyaa.si/ns leechers"`
	InfoHash string `xml:"http://nyaa.si/ns infoHash"`
	Size     string `xml:"http://nyaa.si/ns size"`
}

func (n *NyaaIndexer) Search(ctx context.Context, keyword string) ([]Candidate, error) {
	if n.BaseURL == "" {
		n.BaseURL = "https://nyaa.si"
	}
	if n.Client == nil {
		n.Client = &http.Client{Timeout: 15 * time.Second}
	}

	u := n.BaseURL + "/?page=rss&q=" + url.QueryEscape(keyword) + "&c=1_2&f=0"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anidog/1.0)")

	resp, err := n.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nyaa 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nyaa 返回状态码 %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nyaa 读取响应失败: %w", err)
	}

	// 由于 Go XML 对命名空间处理的怪癖，这里用 loose parser
	items, err := parseNyaaRSS(data)
	if err != nil {
		return nil, fmt.Errorf("nyaa 解析 RSS 失败: %w", err)
	}

	out := make([]Candidate, 0, len(items))
	for _, it := range items {
		c := Candidate{
			Title:      it.Title,
			TorrentURL: it.Link,
			InfoHash:   strings.ToUpper(it.InfoHash),
			Size:       parseHumanSize(it.Size),
			Seeders:    parseIntSafe(it.Seeders),
			Leechers:   parseIntSafe(it.Leechers),
			SourceName: "nyaa",
			DetailURL:  it.GUID,
		}
		// 从 InfoHash 合成 magnet
		if c.InfoHash != "" {
			c.MagnetURL = "magnet:?xt=urn:btih:" + strings.ToLower(c.InfoHash)
		}
		if t, err := parseRSSDate(it.PubDate); err == nil {
			c.PubDate = t
		}
		out = append(out, c)
	}
	return out, nil
}

// parseNyaaRSS 不依赖 xmlns 声明，只按 local name 匹配扩展字段
func parseNyaaRSS(data []byte) ([]nyaaItem, error) {
	type rawItem struct {
		XMLName xml.Name
		Title   string `xml:"title"`
		Link    string `xml:"link"`
		GUID    string `xml:"guid"`
		PubDate string `xml:"pubDate"`
		// any: 所有其他元素（含 nyaa:*）
		Any []struct {
			XMLName xml.Name
			Value   string `xml:",chardata"`
		} `xml:",any"`
	}
	type rawChan struct {
		Items []rawItem `xml:"item"`
	}
	type rawRSS struct {
		XMLName xml.Name
		Channel rawChan `xml:"channel"`
	}

	var r rawRSS
	if err := xml.Unmarshal(data, &r); err != nil {
		return nil, err
	}

	out := make([]nyaaItem, 0, len(r.Channel.Items))
	for _, it := range r.Channel.Items {
		ni := nyaaItem{
			Title:   it.Title,
			Link:    it.Link,
			GUID:    it.GUID,
			PubDate: it.PubDate,
		}
		for _, any := range it.Any {
			switch any.XMLName.Local {
			case "seeders":
				ni.Seeders = any.Value
			case "leechers":
				ni.Leechers = any.Value
			case "infoHash":
				ni.InfoHash = any.Value
			case "size":
				ni.Size = any.Value
			}
		}
		out = append(out, ni)
	}
	return out, nil
}

func parseRSSDate(s string) (time.Time, error) {
	// RFC1123 / RFC1123Z
	if t, err := time.Parse(time.RFC1123Z, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC1123, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unknown format: %s", s)
}
