package mediacheck

import (
	"reflect"
	"testing"
)

func TestNativeSubtitleLabelsAcrossContainers(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags map[string]string
		want []string
	}{
		{"mp4 name", map[string]string{"language": "chi", "name": "中文（简体）", "handler_name": "SubtitleHandler"}, []string{"simplified"}},
		{"mp4 traditional", map[string]string{"language": "zho", "name": "中文（繁體）"}, []string{"traditional"}},
		{"mkv title", map[string]string{"language": "chi", "title": "简日双语"}, []string{"simplified"}},
		{"localized handler", map[string]string{"handler_name": "Chinese Simplified"}, []string{"simplified"}},
		{"ietf simplified", map[string]string{"language": "ZH_HANS_CN"}, []string{"simplified"}},
		{"ietf traditional", map[string]string{"language": "zh-Hant-HK"}, []string{"traditional"}},
		{"china code", map[string]string{"language": "zh-CN"}, []string{"simplified"}},
		{"localized china code", map[string]string{"language": "chi", "title": "中文（中国）"}, []string{"simplified"}},
		{"bare Chinese", map[string]string{"language": "chi", "name": "中文", "handler_name": "SubtitleHandler"}, nil},
		{"bare zho", map[string]string{"language": "zho"}, nil},
		{"generic handler", map[string]string{"language": "und", "handler_name": "SubtitleHandler"}, nil},
		{"embedded token is not a language", map[string]string{"language": "fra", "name": "FrenchSubtitles"}, nil},
		{"english", map[string]string{"language": "eng"}, []string{"english"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SubtitleLanguages(tc.tags); !reflect.DeepEqual(got, tc.want) {
				t.Fatal(got, tc.want)
			}
		})
	}
}
