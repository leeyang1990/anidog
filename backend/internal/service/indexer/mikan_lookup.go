package indexer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// LookupMikanBangumiID 用关键词搜 Mikan 番剧主页，找最匹配的 mikan_bangumi_id。
//
// 实测覆盖率 ~80%：
//   - 中文官名 / 罗马音 / 英文官名 → 多数命中
//   - 中文意译名 / 日文原名 → 可能漏（调用方应先 Title 后 OriginalTitle 各试一次）
//
// 返回 0 表示没找到，调用方应回退到关键词 indexer 流程。
//
// 匹配策略：
//  1. 完全匹配 keyword 优先
//  2. 否则取第一条（Mikan 默认按相关性排）
//
// season 参数（如 1/2/3）用于多季作品消歧；为 0 时不参与匹配。
func LookupMikanBangumiID(ctx context.Context, keyword string, season int, clients ...*http.Client) (int, string, error) {
	if strings.TrimSpace(keyword) == "" {
		return 0, "", nil
	}

	form := url.Values{}
	form.Set("searchstr", keyword)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://mikanani.me/Home/Search",
		strings.NewReader(form.Encode()))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anidog/1.0)")

	client := &http.Client{Timeout: 10 * time.Second}
	if len(clients) > 0 && clients[0] != nil {
		client = clients[0]
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("mikan search 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("mikan search HTTP %d", resp.StatusCode)
	}

	bodyBytes := make([]byte, 0, 1<<20)
	buf := make([]byte, 4096)
	for {
		n, _ := resp.Body.Read(buf)
		if n > 0 {
			bodyBytes = append(bodyBytes, buf[:n]...)
		}
		if n == 0 {
			break
		}
	}
	body := string(bodyBytes)

	// 匹配 <a href="/Home/Bangumi/3938" target="_blank"> ... title="异世界悠闲农家 第二季"
	pattern := regexp.MustCompile(`<a href="/Home/Bangumi/(\d+)" target="_blank">[\s\S]*?title="([^"]+)"`)
	matches := pattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return 0, "", nil
	}

	type cand struct {
		ID    int
		Title string
	}
	cands := make([]cand, 0, len(matches))
	for _, m := range matches {
		var id int
		fmt.Sscanf(m[1], "%d", &id)
		t := unescapeHTMLEntities(m[2])
		cands = append(cands, cand{ID: id, Title: t})
	}

	// 多季消歧：按 season 选包含 "第N季" / "S2" 等关键词的
	if season > 1 {
		for _, c := range cands {
			if seasonMatches(c.Title, season) {
				return c.ID, c.Title, nil
			}
		}
	}
	// 季度=1：优先选不含 "第N季"（N>=2）的
	if season == 1 {
		for _, c := range cands {
			if !hasHigherSeason(c.Title) {
				return c.ID, c.Title, nil
			}
		}
	}

	// 兜底：完全匹配 keyword
	for _, c := range cands {
		if strings.EqualFold(c.Title, keyword) {
			return c.ID, c.Title, nil
		}
	}

	// 否则取第一条
	return cands[0].ID, cands[0].Title, nil
}

func seasonMatches(title string, season int) bool {
	t := strings.ToLower(title)
	suffixes := []string{
		fmt.Sprintf("第%d季", season),
		fmt.Sprintf("第%s季", chineseNumeral(season)),
		fmt.Sprintf("s%d", season),
		fmt.Sprintf("season %d", season),
		fmt.Sprintf("%d期", season),
	}
	for _, s := range suffixes {
		if strings.Contains(t, strings.ToLower(s)) {
			return true
		}
	}
	// 罗马音 + 数字
	if season > 1 {
		if strings.HasSuffix(strings.TrimSpace(t), fmt.Sprintf(" %d", season)) {
			return true
		}
	}
	return false
}

func hasHigherSeason(title string) bool {
	for s := 2; s <= 9; s++ {
		if seasonMatches(title, s) {
			return true
		}
	}
	return false
}

func chineseNumeral(n int) string {
	m := map[int]string{1: "一", 2: "二", 3: "三", 4: "四", 5: "五", 6: "六", 7: "七", 8: "八", 9: "九"}
	if v, ok := m[n]; ok {
		return v
	}
	return fmt.Sprintf("%d", n)
}

func unescapeHTMLEntities(s string) string {
	// Mikan 用十六进制 entity，&#x871C; → 蜜
	re := regexp.MustCompile(`&#x([0-9a-fA-F]+);`)
	out := re.ReplaceAllStringFunc(s, func(match string) string {
		hex := match[3 : len(match)-1]
		var n int
		fmt.Sscanf(hex, "%x", &n)
		if n > 0 {
			return string(rune(n))
		}
		return match
	})
	// 常见 entity
	out = strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
		"&nbsp;", " ",
	).Replace(out)
	return out
}
