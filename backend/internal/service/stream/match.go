package stream

import (
	"sort"
	"strings"
)

// MatchScore 计算候选标题与目标标题的匹配分数。越高越好。
//
// 评分规则：
//   - 季度数字一致: +1000（强匹配，防止"第二季"搜出"第四季"被选中）
//   - 候选标题包含目标全部非季度字符串: +300
//   - 目标标题包含候选全部非季度字符串: +200
//   - 基础前缀/子串重叠长度（字符级）: 加权分
func MatchScore(target, candidate string) int {
	score := 0

	// 1. 季度精确匹配（核心规则）
	ts := detectSeason(target)
	cs := detectSeason(candidate)
	if ts == cs {
		score += 1000
	} else {
		// 季度不同直接大幅扣分，防止误选
		score -= 500
	}

	// 2. 基础名（去季度后缀后）包含关系
	tBase := stripSeasonSuffix(target)
	cBase := stripSeasonSuffix(candidate)
	if tBase != "" && cBase != "" {
		if strings.Contains(cBase, tBase) {
			score += 300
		} else if strings.Contains(tBase, cBase) {
			score += 200
		}
	}

	// 3. 字符级重叠长度加分（LCS 的简化版：共同出现的 rune）
	overlap := runeOverlapLen(tBase, cBase)
	score += overlap * 10

	return score
}

// stripSeasonSuffix 去掉「第X季」「Season X」等季度后缀
func stripSeasonSuffix(s string) string {
	out := s
	for _, pat := range seasonPatterns {
		out = pat.ReplaceAllString(out, "")
	}
	return strings.TrimSpace(out)
}

// runeOverlapLen 返回两字符串中共同出现的 rune 数量（粗略相似度）
func runeOverlapLen(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	set := make(map[rune]bool)
	for _, r := range a {
		set[r] = true
	}
	count := 0
	seen := make(map[rune]bool)
	for _, r := range b {
		if set[r] && !seen[r] {
			count++
			seen[r] = true
		}
	}
	return count
}

// PickBestMatch 在结果中选出与目标标题最匹配的一个。空列表返回 nil。
func PickBestMatch(target string, results []SearchResult) *SearchResult {
	if len(results) == 0 {
		return nil
	}
	best := &results[0]
	bestScore := MatchScore(target, best.Name)
	for i := 1; i < len(results); i++ {
		s := MatchScore(target, results[i].Name)
		if s > bestScore {
			bestScore = s
			best = &results[i]
		}
	}
	return best
}

// SortResultsByMatch 按匹配度降序排序（就地修改）
func SortResultsByMatch(target string, results []SearchResult) {
	sort.SliceStable(results, func(i, j int) bool {
		return MatchScore(target, results[i].Name) > MatchScore(target, results[j].Name)
	})
}
