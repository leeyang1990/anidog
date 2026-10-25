package mediacheck

import "strings"

// SubtitleLanguages interprets the actual subtitle stream's metadata. MP4
// commonly exposes a track label as name or handler_name, while MKV uses title.
// Bare chi/zho/zh identifies Chinese but cannot establish its writing system.
func SubtitleLanguages(tags map[string]string) []string {
	language := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(tags["language"])), "_", "-")
	label := strings.ToLower(strings.Join([]string{tags["title"], tags["name"], tags["handler_name"]}, " "))
	simplified := language == "chs" || language == "hans" || language == "zh-chs" || language == "zh-cn" || language == "zh-sg" || strings.HasPrefix(language, "zh-hans") || strings.HasPrefix(language, "cmn-hans")
	traditional := language == "cht" || language == "hant" || language == "zh-cht" || language == "zh-tw" || language == "zh-hk" || language == "zh-mo" || strings.HasPrefix(language, "zh-hant") || strings.HasPrefix(language, "cmn-hant")
	for _, token := range strings.FieldsFunc(label, func(r rune) bool { return r < 'a' || r > 'z' }) {
		switch token {
		case "chs", "hans", "simplified":
			simplified = true
		case "cht", "hant", "traditional":
			traditional = true
		}
	}
	for _, marker := range []string{"简", "簡", "中国"} {
		simplified = simplified || strings.Contains(label, marker)
	}
	for _, marker := range []string{"繁", "香港", "台灣"} {
		traditional = traditional || strings.Contains(label, marker)
	}
	var result []string
	if simplified {
		result = append(result, "simplified")
	}
	if traditional {
		result = append(result, "traditional")
	}
	if language == "jpn" || language == "ja" {
		result = append(result, "japanese")
	}
	if language == "eng" || language == "en" {
		result = append(result, "english")
	}
	return result
}
