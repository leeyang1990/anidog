package mediapolicy

import (
	"github.com/anidog/anidog-go/internal/model"
	"reflect"
	"testing"
)

func TestGoalContract(t *testing.T) {
	for _, tc := range []struct {
		name, global, override string
		want                   []string
		invalid                bool
	}{
		{"default", "", "", []string{"simplified"}, false},
		{"global unrestricted", "[]", "", []string{}, false},
		{"empty override inherits", `["simplified"]`, "[]", []string{"simplified"}, false},
		{"override", `["traditional"]`, `["simplified","simplified"]`, []string{"simplified"}, false},
		{"invalid global", "broken", "", nil, true},
		{"null global", "null", "", nil, true},
		{"invalid override", `["simplified"]`, "broken", nil, true},
		{"unknown language", `["chs"]`, "", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{"download.languages": tc.global}
			anime := &model.Anime{OverrideLanguages: &tc.override}
			g, err := Resolve(values, anime)
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v", err)
			}
			if !tc.invalid && (g.Quality != "1080p" || !reflect.DeepEqual(g.Languages, tc.want) || g.AllowFallback) {
				t.Fatal(g)
			}
		})
	}
	if _, err := Resolve(map[string]string{"download.quality": "garbage"}, nil); err == nil {
		t.Fatal("invalid quality silently disabled checking")
	}
	a := Default()
	b, _ := Apply(a, nil)
	b.Languages[0] = "traditional"
	if a.Languages[0] != "simplified" {
		t.Fatal("policy overrides mutated the global snapshot")
	}
}
