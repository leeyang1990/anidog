// Package titleparse 解析番剧种子/RSS 条目标题，提取字幕组、番名、集数、分辨率、语言等字段。
// 针对中文字幕组的常见命名惯例进行优化，同时兼容英语圈 Nyaa 式命名。
package titleparse

import (
	"regexp"
	"strings"
)

// ParsedTitle 解析结果
type ParsedTitle struct {
	Group      string   `json:"group"`                // 字幕组，如 "LoliHouse"、"桜都字幕组"
	AnimeName  string   `json:"anime_name"`           // 番名（中文优先）
	AltNames   []string `json:"alt_names"`            // 其他名字段（英文/日文罗马音）
	EpisodeNum *int     `json:"episode_num"`          // 集数；批量包时为 nil，见 BatchStart/End
	SeasonNum  *int     `json:"season_num,omitempty"` // 标题明确标注的季度；未知为 nil
	IsBatch    bool     `json:"is_batch"`             // 是否为合集（如 01-12 Fin）
	BatchStart *int     `json:"batch_start,omitempty"`
	BatchEnd   *int     `json:"batch_end,omitempty"`
	Quality    string   `json:"quality"` // "720p" / "1080p" / "2160p" / "4K" 等
	Lang       []string `json:"lang"`    // ["simplified","traditional","japanese","english"]
	Source     string   `json:"source"`  // "WEB-DL" / "BDRip" / "BD" / "TV" / "Baha" 等
	Codec      string   `json:"codec"`   // "HEVC-10bit" / "AVC" / "x264" 等
	Raw        string   `json:"raw"`     // 原始标题
}

var (
	// 方括号 token
	reBracket = regexp.MustCompile(`\[([^\]]+)\]`)

	// 小括号（常见于 Nyaa "(1080p)"）
	reParen = regexp.MustCompile(`\([^\)]*\)`)

	// 文件扩展名
	reFileExt = regexp.MustCompile(`(?i)\.(mkv|mp4|avi|ts|m2ts|flv|mov|webm)$`)

	// 集数模式（优先级从高到低）
	reEpisodeDash      = regexp.MustCompile(`[\s_\-\.]-\s*(\d{1,3}(?:\.5)?)(?:v\d+)?(?:[\s_\-\.\[]|$)`) // " - 05" 或 "- 05.5"
	reEpisodeBracket   = regexp.MustCompile(`\[(\d{1,3}(?:\.5)?)(?:v\d+)?\]`)                           // "[05]"
	reEpisodeChinese   = regexp.MustCompile(`第\s*(\d{1,3})\s*[话集話]`)                                    // "第05话"
	reEpisodeE         = regexp.MustCompile(`\bE[Pp]?(\d{1,3})\b`)                                      // "E05" / "EP05"
	reEpisodeStandalon = regexp.MustCompile(`\s(\d{1,3})(?:v\d+)?\s*(?:END|FIN|完)?\s*$`)                // trailing number

	// 季度模式。只把明确标记视为季度；无法识别时保持 nil，交给上层 fail-open。
	reSeasonChinese = regexp.MustCompile(`第\s*([0-9一二三四五六七八九十]+)\s*[季期]`)
	reSeasonEnglish = regexp.MustCompile(`(?i)\bSeason\s*(\d+)\b|\bS(\d+)(?:E\d+)?\b|\b(\d+)(?:st|nd|rd|th)\s+Season\b`)
	reSeasonRoman   = regexp.MustCompile(`(?i)\b(II|III|IV|V|VI)\b`)

	// 批量包模式
	reBatch = regexp.MustCompile(`(\d{1,3})\s*[-~～]\s*(\d{1,3})(?:\s*(?:END|Fin|FIN|完|全))?`)

	// 分辨率
	reResolutionP = regexp.MustCompile(`\b(480|576|720|900|1080|1440|2160|4320)[pPiI]\b`)
	reResolutionK = regexp.MustCompile(`\b(4K|8K|FHD|UHD)\b`)

	// 来源
	reSource = regexp.MustCompile(`(?i)\b(WEB-?DL|WEB-?Rip|WebRip|Blu-?Ray|BD-?Rip|BDRip|BD-?MV|BDBox|BD|HDTV|TV|DVD-?Rip|DVDRip|DVD|Baha|Bilibili|CR|Crunchyroll|NF|Netflix|AMZN|Amazon|KKTV|iQIYI|AT-X)\b`)

	// 编码
	reCodec       = regexp.MustCompile(`(?i)\b(HEVC|H\.?265|x265|AVC|H\.?264|x264|AV1)\b(?:[\s_\-](?:10-?bit|10bit))?`)
	reCodecWith10 = regexp.MustCompile(`(?i)\b(HEVC|H\.?265|x265)[\s_\-]*10-?bit\b`)

	// 废弃 token 过滤（不作为字幕组）
	groupBlacklist = map[string]bool{
		"1080P": true, "1080p": true, "720P": true, "720p": true, "2160P": true, "2160p": true,
		"4K": true, "8K": true, "FHD": true, "UHD": true,
		"HEVC": true, "AVC": true, "x264": true, "x265": true, "H264": true, "H265": true,
		"WEB-DL": true, "WEBRIP": true, "BDRIP": true, "BD": true, "TV": true, "DVDRIP": true,
		"CHS": true, "CHT": true, "BIG5": true, "GB": true, "JPN": true, "ENG": true,
		"简中": true, "繁中": true, "简体": true, "繁体": true, "日语": true, "英语": true,
		"简繁内封": true, "简繁内封字幕": true, "简繁外挂": true, "简体内嵌": true, "繁体内嵌": true,
		"内封": true, "外挂": true, "内嵌": true,
		"MP4": true, "MKV": true, "AAC": true, "AC3": true, "FLAC": true, "DTS": true,
		"AAC AVC": true, "AAC HEVC": true,
		"Baha": true, "CR": true, "NF": true, "AMZN": true, "KKTV": true, "AT-X": true, "Bilibili": true,
	}
)

// Parse 解析标题字符串，返回结构化结果。
// 任何字段解析失败都不会让整个解析失败（各字段独立）。
func Parse(title string) *ParsedTitle {
	if title == "" {
		return &ParsedTitle{Raw: title}
	}

	p := &ParsedTitle{Raw: title}
	p.SeasonNum = detectSeasonNumber(title)

	// 预处理：把 `_` 替换为空格以便正则 \b 边界生效；保留原始标题用于个别正则
	normalized := strings.ReplaceAll(title, "_", " ")

	// 1. 切出所有方括号 token（用原始 title，因为方括号内可能有下划线）
	brackets := reBracket.FindAllStringSubmatch(title, -1)
	var tokens []string
	for _, m := range brackets {
		tokens = append(tokens, strings.TrimSpace(m[1]))
	}

	// 2. 去掉所有 [...] 后得到"核心文本"，用来提取番名和集数
	core := reBracket.ReplaceAllString(title, " ")
	// 去掉 (...) 小括号（常见于 Nyaa "(1080p)"）
	core = reParen.ReplaceAllString(core, " ")
	// 去掉文件扩展名
	core = reFileExt.ReplaceAllString(core, "")
	core = strings.Join(strings.Fields(core), " ")

	// 3. 识别字幕组（第一个非黑名单的方括号 token）
	for _, t := range tokens {
		if t == "" {
			continue
		}
		upper := strings.ToUpper(t)
		if groupBlacklist[t] || groupBlacklist[upper] {
			continue
		}
		// 含集数/分辨率/编码的 token 跳过
		tNorm := strings.ReplaceAll(t, "_", " ")
		if reResolutionP.MatchString(tNorm) || reResolutionK.MatchString(tNorm) {
			continue
		}
		if reCodec.MatchString(tNorm) && !containsChinese(t) {
			continue
		}
		if reEpisodeBracket.MatchString("[" + t + "]") {
			continue
		}
		// 过滤纯数字/纯集数标记
		if isNumericToken(t) {
			continue
		}
		p.Group = t
		break
	}

	// 4. 分辨率（用 normalized 字符串做匹配）
	if m := reResolutionP.FindStringSubmatch(normalized); m != nil {
		p.Quality = m[1] + "p"
	} else if m := reResolutionK.FindStringSubmatch(normalized); m != nil {
		p.Quality = strings.ToUpper(m[1])
	}

	// 5. 来源
	if m := reSource.FindStringSubmatch(normalized); m != nil {
		p.Source = normalizeSource(m[1])
	}

	// 6. 编码（优先匹配 "HEVC 10bit" 组合）
	if m := reCodecWith10.FindStringSubmatch(normalized); m != nil {
		p.Codec = normalizeCodec(m[1]) + "-10bit"
	} else if m := reCodec.FindStringSubmatch(normalized); m != nil {
		p.Codec = normalizeCodec(m[1])
	}

	// 7. 语言
	p.Lang = detectLanguages(title)

	// 8. 批量包（优先于单集）
	if batch := matchBatch(title); batch != nil {
		p.IsBatch = true
		p.BatchStart = &batch[0]
		p.BatchEnd = &batch[1]
	} else {
		// 9. 单集集数
		if n := matchEpisode(title, core, tokens); n != nil {
			p.EpisodeNum = n
		}
	}

	// 10. 番名（从 core 中剥离集数和无关后缀）
	p.AnimeName, p.AltNames = extractAnimeNames(core, p.EpisodeNum)

	return p
}

func detectSeasonNumber(title string) *int {
	if m := reSeasonChinese.FindStringSubmatch(title); len(m) > 1 {
		if n := parseSeasonNumber(m[1]); n > 0 {
			return &n
		}
	}
	if m := reSeasonEnglish.FindStringSubmatch(title); len(m) > 1 {
		for _, value := range m[1:] {
			if n := parseInt(value); n > 0 {
				return &n
			}
		}
	}
	if m := reSeasonRoman.FindStringSubmatch(title); len(m) > 1 {
		roman := map[string]int{"II": 2, "III": 3, "IV": 4, "V": 5, "VI": 6}
		if n := roman[strings.ToUpper(m[1])]; n > 0 {
			return &n
		}
	}
	return nil
}

func parseSeasonNumber(value string) int {
	if n := parseInt(value); n > 0 {
		return n
	}
	return map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
	}[value]
}

// matchEpisode 按优先级查找集数
func matchEpisode(title, core string, tokens []string) *int {
	// 优先级 1：core 文本中的 " - 05" 模式
	if m := reEpisodeDash.FindStringSubmatch(" " + core + " "); m != nil {
		return parseEpisodeInt(m[1])
	}
	// 优先级 2：第 X 话
	if m := reEpisodeChinese.FindStringSubmatch(title); m != nil {
		return parseEpisodeInt(m[1])
	}
	// 优先级 3：[05] bracket（且不是分辨率）
	for _, t := range tokens {
		if reResolutionP.MatchString(t) || reResolutionK.MatchString(t) {
			continue
		}
		if reEpisodeBracket.MatchString("[" + t + "]") {
			m := reEpisodeBracket.FindStringSubmatch("[" + t + "]")
			return parseEpisodeInt(m[1])
		}
	}
	// 优先级 4：E05 / EP05
	if m := reEpisodeE.FindStringSubmatch(title); m != nil {
		return parseEpisodeInt(m[1])
	}
	// 优先级 5：末尾孤立数字（core 尾部）
	if m := reEpisodeStandalon.FindStringSubmatch(" " + core); m != nil {
		return parseEpisodeInt(m[1])
	}
	return nil
}

// matchBatch 识别批量包（如 01-12 Fin）
func matchBatch(title string) []int {
	matches := reBatch.FindAllStringSubmatch(title, -1)
	for _, m := range matches {
		s := parseInt(m[1])
		e := parseInt(m[2])
		// 合理性检查：end > start 且差值合理（避免把分辨率数字误识别）
		if s > 0 && e > s && e-s <= 200 && s <= 200 && e <= 300 {
			// 必须有明确的合集标识（Fin/END/完/全）或出现在方括号中
			full := m[0]
			if strings.ContainsAny(strings.ToLower(full), "finENDend完全") ||
				bracketContains(title, full) {
				return []int{s, e}
			}
		}
	}
	return nil
}

// bracketContains 判断 needle 是否出现在 title 的某个 [...] 内
func bracketContains(title, needle string) bool {
	for _, m := range reBracket.FindAllStringSubmatch(title, -1) {
		if strings.Contains(m[1], needle) {
			return true
		}
	}
	return false
}

// extractAnimeNames 从 core 文本中提取所有番名候选（中文优先，其他语言作为 alt）
// 例子: "葬送的芙莉莲 / Sousou no Frieren - 03"
//
//	→ main="葬送的芙莉莲", alts=["Sousou no Frieren"]
//
// "RentaGirlfriend S05 / 出租女友 第五季 - 05"
//
//	→ main="出租女友 第五季", alts=["RentaGirlfriend S05"]
func extractAnimeNames(core string, episodeNum *int) (string, []string) {
	s := core

	// 去掉集数 token（" - 05"、"第05话" 等）
	s = reEpisodeDash.ReplaceAllString(" "+s+" ", " ")
	s = reEpisodeChinese.ReplaceAllString(s, " ")
	s = reEpisodeE.ReplaceAllString(s, " ")
	s = reBatch.ReplaceAllString(s, " ")

	// 斜杠分段
	parts := strings.Split(s, "/")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, " -_.·|")
		p = strings.Join(strings.Fields(p), " ")
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return "", nil
	}

	// 主名：优先含 CJK 中文的段
	var mainIdx = -1
	for i, p := range cleaned {
		if containsChinese(p) {
			mainIdx = i
			break
		}
	}
	if mainIdx < 0 {
		// 没中文，取最长的一段做主名
		longest := 0
		for i, p := range cleaned {
			if len(p) > longest {
				longest = len(p)
				mainIdx = i
			}
		}
	}

	main := cleaned[mainIdx]
	alts := make([]string, 0, len(cleaned)-1)
	for i, p := range cleaned {
		if i != mainIdx {
			alts = append(alts, p)
		}
	}
	return main, alts
}

func detectLanguages(title string) []string {
	var langs []string
	has := func(s string) bool {
		return strings.Contains(title, s) || strings.Contains(strings.ToUpper(title), strings.ToUpper(s))
	}
	if has("简") || has("CHS") || has("GB") || has("简中") || has("简体") {
		langs = append(langs, "simplified")
	}
	if has("繁") || has("CHT") || has("BIG5") || has("繁中") || has("繁体") {
		langs = append(langs, "traditional")
	}
	if has("日") && (has("日语") || has("日文") || has("JPN")) {
		langs = append(langs, "japanese")
	}
	if has("英") && (has("英语") || has("英文")) || has("ENG") {
		langs = append(langs, "english")
	}
	return langs
}

func normalizeSource(s string) string {
	u := strings.ToUpper(s)
	u = strings.ReplaceAll(u, "-", "")
	switch u {
	case "WEBDL":
		return "WEB-DL"
	case "WEBRIP":
		return "WebRip"
	case "BDRIP", "BLURAYRIP":
		return "BDRip"
	case "BLURAY", "BD":
		return "BD"
	case "DVDRIP":
		return "DVDRip"
	case "BAHA":
		return "Baha"
	case "BILIBILI":
		return "Bilibili"
	case "CR", "CRUNCHYROLL":
		return "CR"
	case "NF", "NETFLIX":
		return "Netflix"
	case "AMZN", "AMAZON":
		return "Amazon"
	}
	return s
}

func normalizeCodec(s string) string {
	u := strings.ToUpper(strings.ReplaceAll(s, ".", ""))
	switch u {
	case "HEVC", "H265", "X265":
		return "HEVC"
	case "AVC", "H264", "X264":
		return "AVC"
	case "AV1":
		return "AV1"
	}
	return s
}

// containsChinese 判断字符串是否包含 CJK 汉字
func containsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF { // CJK Unified Ideographs
			return true
		}
		if r >= 0x3400 && r <= 0x4DBF { // CJK Ext A
			return true
		}
	}
	return false
}

func isNumericToken(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func parseInt(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func parseEpisodeInt(s string) *int {
	// 处理 .5 半集
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) == 2 {
			n := parseInt(parts[0])
			if n > 0 {
				return &n
			}
		}
	}
	n := parseInt(s)
	if n > 0 {
		return &n
	}
	return nil
}
