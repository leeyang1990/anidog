package rss

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"
)

// ParsedItem represents a single parsed RSS entry.
type ParsedItem struct {
	EntryID   string
	Title     string
	Link      string
	Published *time.Time
}

// Parser fetches and parses RSS feeds.
type Parser struct {
	httpClient *http.Client
}

// NewParser creates a new Parser with a reasonable HTTP timeout.
func NewParser() *Parser {
	return &Parser{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Parse fetches and parses an RSS/Atom feed URL.
// It auto-detects the feed type (mikan vs generic).
func (p *Parser) Parse(ctx context.Context, feedURL string) ([]ParsedItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Anidog/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 RSS 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS 返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 RSS 内容失败: %w", err)
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(body))
	if err != nil {
		return nil, fmt.Errorf("解析 RSS 失败: %w", err)
	}

	var items []ParsedItem
	for _, item := range feed.Items {
		link := item.Link
		// gofeed may put the enclosure URL in Enclosures
		if link == "" && len(item.Enclosures) > 0 {
			link = item.Enclosures[0].URL
		}
		if link == "" {
			continue
		}

		// For mikan, the torrent link is often in the enclosure
		// with type application/x-bittorrent
		if strings.Contains(feedURL, "mikan") && len(item.Enclosures) > 0 {
			for _, enc := range item.Enclosures {
				if strings.Contains(enc.Type, "bittorrent") || strings.HasSuffix(enc.URL, ".torrent") {
					link = enc.URL
					break
				}
			}
		}

		entryID := item.GUID
		if entryID == "" {
			entryID = item.Link
		}
		if entryID == "" {
			entryID = link
		}

		items = append(items, ParsedItem{
			EntryID:   entryID,
			Title:     item.Title,
			Link:      link,
			Published: item.PublishedParsed,
		})
	}

	zap.L().Debug("RSS 解析完成", zap.String("url", feedURL), zap.Int("items", len(items)))
	return items, nil
}
