package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// BangumiMoeIndexer 萌番组
// POST https://bangumi.moe/api/v2/torrent/search  body {"query":"keyword"}
type BangumiMoeIndexer struct {
	BaseURL string
	Client  *http.Client
}

func NewBangumiMoeIndexer() *BangumiMoeIndexer {
	return &BangumiMoeIndexer{
		BaseURL: "https://bangumi.moe",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (b *BangumiMoeIndexer) Name() string { return "bangumimoe" }

type bangumiMoeTorrent struct {
	ID          string    `json:"_id"`
	Title       string    `json:"title"`
	Magnet      string    `json:"magnet"`
	InfoHash    string    `json:"infoHash"`
	PublishTime time.Time `json:"publish_time"`
	Size        string    `json:"size"`
	Seeders     int       `json:"seeders"`
	Leechers    int       `json:"leechers"`
	Downloads   int       `json:"downloads"`
	TeamID      *string   `json:"team_id"`
}

type bangumiMoeSearchResp struct {
	Torrents []bangumiMoeTorrent `json:"torrents"`
}

func (b *BangumiMoeIndexer) Search(ctx context.Context, keyword string) ([]Candidate, error) {
	if b.BaseURL == "" {
		b.BaseURL = "https://bangumi.moe"
	}
	if b.Client == nil {
		b.Client = &http.Client{Timeout: 15 * time.Second}
	}

	body, _ := json.Marshal(map[string]string{"query": keyword})
	req, err := http.NewRequestWithContext(ctx, "POST",
		b.BaseURL+"/api/v2/torrent/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anidog/1.0)")

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bangumimoe 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bangumimoe 返回状态码 %d", resp.StatusCode)
	}

	var payload bangumiMoeSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("bangumimoe 解析 JSON 失败: %w", err)
	}

	out := make([]Candidate, 0, len(payload.Torrents))
	for _, t := range payload.Torrents {
		// bangumi.moe API v2 在搜索接口里不返回有效 seeders/leechers（永远是 0），
		// 不能当成"已知 0 活种"硬否决；交给 scrape 探活去判断。
		c := Candidate{
			Title:      t.Title,
			MagnetURL:  t.Magnet,
			InfoHash:   strings.ToUpper(t.InfoHash),
			PubDate:    t.PublishTime,
			Size:       parseHumanSize(t.Size),
			Seeders:    0,
			Leechers:   0,
			SeedersReported: false,
			SourceName: "bangumimoe",
			DetailURL:  b.BaseURL + "/torrent/" + t.ID,
		}
		out = append(out, c)
	}
	return out, nil
}
