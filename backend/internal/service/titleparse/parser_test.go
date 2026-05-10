package titleparse

import (
	"testing"
)

func intPtr(n int) *int { return &n }

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  ParsedTitle
	}{
		{
			name:  "LoliHouse 标准格式",
			title: "[LoliHouse] 葬送的芙莉莲 / Sousou no Frieren - 03 [WebRip 1080p HEVC-10bit AAC][简繁内封字幕]",
			want: ParsedTitle{
				Group:      "LoliHouse",
				AnimeName:  "葬送的芙莉莲",
				EpisodeNum: intPtr(3),
				Quality:    "1080p",
				Source:     "WebRip",
				Codec:      "HEVC-10bit",
				Lang:       []string{"simplified", "traditional"},
			},
		},
		{
			name:  "ANi 格式 + 斜杠反序",
			title: "[ANi] Isekai Nonbiri Nōka /  异世界悠闲农家 2 - 05 [1080P][Baha][WEB-DL][AAC AVC][CHT][MP4]",
			want: ParsedTitle{
				Group:      "ANi",
				AnimeName:  "异世界悠闲农家 2",
				EpisodeNum: intPtr(5),
				Quality:    "1080p",
				Source:     "Baha",
				Codec:      "AVC",
				Lang:       []string{"traditional"},
			},
		},
		{
			name:  "桜都字幕组方括号集数",
			title: "[桜都字幕组] 冰之城墙 / Koori no Jouheki [05][1080P][简繁内封]",
			want: ParsedTitle{
				Group:      "桜都字幕组",
				AnimeName:  "冰之城墙",
				EpisodeNum: intPtr(5),
				Quality:    "1080p",
				Lang:       []string{"simplified", "traditional"},
			},
		},
		{
			name:  "批量包 Fin",
			title: "[桜都字幕组] 有栖川炼原来是女孩子啊。 / Arisugawa Ren tte Honto wa Onna Nanda yo ne. [01-08 Fin][1080P][简繁内封]",
			want: ParsedTitle{
				Group:      "桜都字幕组",
				AnimeName:  "有栖川炼原来是女孩子啊。",
				IsBatch:    true,
				BatchStart: intPtr(1),
				BatchEnd:   intPtr(8),
				Quality:    "1080p",
				Lang:       []string{"simplified", "traditional"},
			},
		},
		{
			name:  "ANi 季数带在番名中",
			title: "[ANi] Re:从零开始的异世界生活 第四季 - 04 [1080P][Baha][WEB-DL][AAC AVC][CHT][MP4]",
			want: ParsedTitle{
				Group:      "ANi",
				AnimeName:  "Re:从零开始的异世界生活 第四季",
				EpisodeNum: intPtr(4),
				Quality:    "1080p",
				Source:     "Baha",
				Codec:      "AVC",
				Lang:       []string{"traditional"},
			},
		},
		{
			name:  "简体内嵌",
			title: "[桜都字幕组] 这样高大的女孩子你喜欢吗？ / Ookii Onnanoko wa Suki Desuka？ [04][1080P][简体内嵌]",
			want: ParsedTitle{
				Group:      "桜都字幕组",
				AnimeName:  "这样高大的女孩子你喜欢吗？",
				EpisodeNum: intPtr(4),
				Quality:    "1080p",
				Lang:       []string{"simplified"},
			},
		},
		{
			name:  "繁体内嵌",
			title: "[桜都字幕组] 这样高大的女孩子你喜欢吗？ / Ookii Onnanoko wa Suki Desuka？ [04][1080P][繁体内嵌]",
			want: ParsedTitle{
				Group:      "桜都字幕组",
				AnimeName:  "这样高大的女孩子你喜欢吗？",
				EpisodeNum: intPtr(4),
				Quality:    "1080p",
				Lang:       []string{"traditional"},
			},
		},
		{
			name:  "E05 格式",
			title: "[喵萌奶茶屋] 葬送的芙莉莲 E05 [1080p]",
			want: ParsedTitle{
				Group:      "喵萌奶茶屋",
				AnimeName:  "葬送的芙莉莲",
				EpisodeNum: intPtr(5),
				Quality:    "1080p",
			},
		},
		{
			name:  "第XX话 格式",
			title: "[某字幕组] 某番剧 第12话 [1080p]",
			want: ParsedTitle{
				Group:      "某字幕组",
				AnimeName:  "某番剧",
				EpisodeNum: intPtr(12),
				Quality:    "1080p",
			},
		},
		{
			name:  "Nyaa 风格",
			title: "[SubsPlease] Frieren - 03 (1080p) [ABCD1234].mkv",
			want: ParsedTitle{
				Group:      "SubsPlease",
				AnimeName:  "Frieren",
				EpisodeNum: intPtr(3),
				Quality:    "1080p",
			},
		},
		{
			name:  "BDRip + x265",
			title: "[VCB-Studio] 葬送的芙莉莲 [01][Ma10p_1080p][x265_flac].mkv",
			want: ParsedTitle{
				Group:      "VCB-Studio",
				AnimeName:  "葬送的芙莉莲",
				EpisodeNum: intPtr(1),
				Quality:    "1080p",
				Codec:      "HEVC",
			},
		},
		{
			name:  "空标题",
			title: "",
			want:  ParsedTitle{Raw: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.title)

			if got.Group != tt.want.Group {
				t.Errorf("Group: got %q, want %q", got.Group, tt.want.Group)
			}
			if got.AnimeName != tt.want.AnimeName {
				t.Errorf("AnimeName: got %q, want %q", got.AnimeName, tt.want.AnimeName)
			}
			if !equalIntPtr(got.EpisodeNum, tt.want.EpisodeNum) {
				t.Errorf("EpisodeNum: got %v, want %v", ptrStr(got.EpisodeNum), ptrStr(tt.want.EpisodeNum))
			}
			if got.IsBatch != tt.want.IsBatch {
				t.Errorf("IsBatch: got %v, want %v", got.IsBatch, tt.want.IsBatch)
			}
			if tt.want.IsBatch {
				if !equalIntPtr(got.BatchStart, tt.want.BatchStart) {
					t.Errorf("BatchStart: got %v, want %v", ptrStr(got.BatchStart), ptrStr(tt.want.BatchStart))
				}
				if !equalIntPtr(got.BatchEnd, tt.want.BatchEnd) {
					t.Errorf("BatchEnd: got %v, want %v", ptrStr(got.BatchEnd), ptrStr(tt.want.BatchEnd))
				}
			}
			if got.Quality != tt.want.Quality {
				t.Errorf("Quality: got %q, want %q", got.Quality, tt.want.Quality)
			}
			if tt.want.Source != "" && got.Source != tt.want.Source {
				t.Errorf("Source: got %q, want %q", got.Source, tt.want.Source)
			}
			if tt.want.Codec != "" && got.Codec != tt.want.Codec {
				t.Errorf("Codec: got %q, want %q", got.Codec, tt.want.Codec)
			}
			if !equalStrSlice(got.Lang, tt.want.Lang) {
				t.Errorf("Lang: got %v, want %v", got.Lang, tt.want.Lang)
			}
		})
	}
}

func TestContainsChinese(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"hello", false},
		{"葬送的芙莉莲", true},
		{"Re:从零开始", true},
		{"123", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := containsChinese(tt.s); got != tt.want {
			t.Errorf("containsChinese(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestDetectLanguages(t *testing.T) {
	tests := []struct {
		title string
		want  []string
	}{
		{"[组] 番 [简繁内封字幕]", []string{"simplified", "traditional"}},
		{"[组] 番 [简体内嵌]", []string{"simplified"}},
		{"[组] 番 [繁体内嵌]", []string{"traditional"}},
		{"[组] 番 [CHT]", []string{"traditional"}},
		{"[组] 番 [CHS]", []string{"simplified"}},
		{"[组] 番 [1080p]", nil},
	}
	for _, tt := range tests {
		got := detectLanguages(tt.title)
		if !equalStrSlice(got, tt.want) {
			t.Errorf("detectLanguages(%q) = %v, want %v", tt.title, got, tt.want)
		}
	}
}

// helpers

func equalIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ptrStr(p *int) string {
	if p == nil {
		return "<nil>"
	}
	return itoa(*p)
}

func itoa(n int) string {
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
