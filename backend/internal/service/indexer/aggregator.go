package indexer

import (
	"context"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/anidog/anidog-go/internal/service/titleparse"
)

// Aggregate 并发调用所有 indexers，汇总结果，解析标题，按 InfoHash 去重。
// 默认每个 indexer 超时 10 秒；整体超时依赖 ctx。
func Aggregate(ctx context.Context, indexers []Indexer, keyword string) []Candidate {
	if len(indexers) == 0 {
		return nil
	}

	var (
		mu     sync.Mutex
		merged []Candidate
	)

	g, gctx := errgroup.WithContext(ctx)
	for _, idx := range indexers {
		idx := idx
		g.Go(func() error {
			subCtx, cancel := context.WithTimeout(gctx, 10*time.Second)
			defer cancel()
			items, err := idx.Search(subCtx, keyword)
			if err != nil {
				zap.L().Warn("indexer 搜索失败",
					zap.String("indexer", idx.Name()),
					zap.String("keyword", keyword),
					zap.Error(err))
				return nil // 单 indexer 失败不阻塞整体
			}
			mu.Lock()
			for i := range items {
				if items[i].SourceName == "" {
					items[i].SourceName = idx.Name()
				}
				merged = append(merged, items[i])
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	// 按 InfoHash 去重（InfoHash 空的用 Title+Size 作为 fallback key）
	seen := make(map[string]bool)
	deduped := merged[:0]
	for _, c := range merged {
		key := c.InfoHash
		if key == "" {
			key = c.Title + "|" + itoa64(c.Size)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, c)
	}

	// 解析标题
	for i := range deduped {
		deduped[i].Parsed = titleparse.Parse(deduped[i].Title)
	}

	return deduped
}

// RankByPreference 按偏好评分并排序候选。
// 如 targetEpisode > 0：过滤掉集数不匹配（非批量包）的条目；返回按 Score 降序的结果。
// prefs 任意字段为空/零值 = 不约束。
func RankByPreference(cands []Candidate, prefs DownloadPreference, targetEpisode int) []ScoredCandidate {
	out := make([]ScoredCandidate, 0, len(cands))
	for _, c := range cands {
		// 集数过滤
		if targetEpisode > 0 && c.Parsed != nil {
			matched := false
			if c.Parsed.IsBatch && c.Parsed.BatchStart != nil && c.Parsed.BatchEnd != nil {
				if targetEpisode >= *c.Parsed.BatchStart && targetEpisode <= *c.Parsed.BatchEnd {
					matched = true
				}
			} else if c.Parsed.EpisodeNum != nil && *c.Parsed.EpisodeNum == targetEpisode {
				matched = true
			}
			if !matched {
				continue
			}
		}

		score, reason := scoreOne(c, prefs)
		out = append(out, ScoredCandidate{
			Candidate: c,
			Score:     score,
			Reason:    reason,
		})
	}

	// 按 Score 降序
	sortScored(out)
	return out
}

// ScoreReason 记录单条候选的打分明细（供诊断面板展示）
type ScoreReason struct {
	Passed bool
	Detail string
}

func scoreOne(c Candidate, p DownloadPreference) (float64, []string) {
	var score float64
	var reasons []string

	parsed := c.Parsed
	if parsed == nil {
		reasons = append(reasons, "标题解析失败")
		return 0, reasons
	}

	// 字幕组白名单
	if len(p.Groups) > 0 {
		hit := false
		for _, g := range p.Groups {
			if parsed.Group == g || strings.EqualFold(parsed.Group, g) {
				hit = true
				break
			}
		}
		if hit {
			score += 100
			reasons = append(reasons, "字幕组命中白名单 +100")
		} else {
			score -= 20
			reasons = append(reasons, "字幕组不在白名单 -20")
		}
	}

	// 分辨率
	if p.Quality != "" {
		if parsed.Quality == p.Quality {
			score += 50
			reasons = append(reasons, "分辨率匹配 +50")
		} else if parsed.Quality != "" {
			// 附近分辨率也给一点分（1080 vs 720 相近）
			if closeQuality(parsed.Quality, p.Quality) {
				score += 10
				reasons = append(reasons, "分辨率接近 +10")
			} else {
				score -= 15
				reasons = append(reasons, "分辨率不符 -15")
			}
		}
	}

	// 语言
	if len(p.Languages) > 0 {
		langHit := false
		for _, want := range p.Languages {
			for _, have := range parsed.Lang {
				if want == have {
					langHit = true
					break
				}
			}
		}
		if langHit {
			score += 30
			reasons = append(reasons, "语言匹配 +30")
		} else if len(parsed.Lang) > 0 {
			score -= 10
			reasons = append(reasons, "语言不符 -10")
		}
	}

	// 体积（靠近中位数的加分，超出范围扣分）
	if c.Size > 0 {
		sizeMB := c.Size / 1024 / 1024
		if p.MinSizeMB > 0 && sizeMB < int64(p.MinSizeMB) {
			score -= 30
			reasons = append(reasons, "体积过小 -30")
		}
		if p.MaxSizeMB > 0 && sizeMB > int64(p.MaxSizeMB) {
			score -= 30
			reasons = append(reasons, "体积过大 -30")
		}
	}

	// 种子数（log10 加分，避免大数值过度加权）
	if c.Seeders > 0 {
		add := math.Log10(float64(c.Seeders)+1) * 5
		score += add
	}

	return score, reasons
}

// closeQuality 判断分辨率是否"接近"
func closeQuality(a, b string) bool {
	// 提取数字部分
	num := func(s string) int {
		re := regexp.MustCompile(`\d+`)
		m := re.FindString(s)
		n := 0
		for _, r := range m {
			n = n*10 + int(r-'0')
		}
		return n
	}
	ai, bi := num(a), num(b)
	if ai == 0 || bi == 0 {
		return false
	}
	diff := ai - bi
	if diff < 0 {
		diff = -diff
	}
	return diff <= 480 // 720/1080 距离 360；720/1440 距离 720 算接近；超过就不算
}

func sortScored(s []ScoredCandidate) {
	// 简单冒泡，数据量一般 <50
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j].Score > s[i].Score {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		return "-" + string(buf)
	}
	return string(buf)
}
