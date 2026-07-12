package model

import "testing"

func intPtr(v int) *int          { return &v }
func stringPtr(v string) *string { return &v }

func TestNormalizeSeriesTitle(t *testing.T) {
	tests := map[string]string{
		"幼女战记 第二季":         "幼女战记",
		"碧蓝之海 第3季":         "碧蓝之海",
		"某科学的超电磁炮 第三期":     "某科学的超电磁炮",
		"Re:Zero Season 2": "Re:Zero",
		"Anime 3rd Season": "Anime",
		"86 -不存在的战区-":      "86 -不存在的战区-",
	}
	for input, want := range tests {
		if got := NormalizeSeriesTitle(input); got != want {
			t.Errorf("NormalizeSeriesTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSeasonTitleNormalization(t *testing.T) {
	if got := InferSeasonNumber("幼女战记 第二季"); got != 2 {
		t.Fatalf("season = %d", got)
	}
	if got := CanonicalSeasonTitle("幼女战记", 2); got != "幼女战记 第2季" {
		t.Fatalf("canonical title = %q", got)
	}
}

func TestMediaSeriesIdentity(t *testing.T) {
	a := &Anime{Title: "幼女战记 第二季", Year: intPtr(2026), Season: intPtr(2)}
	if got := a.MediaSeriesTitle(); got != "幼女战记" {
		t.Fatalf("title = %q", got)
	}
	if got := a.MediaSeriesYear(); got != 0 {
		t.Fatalf("unmapped sequel year = %d, want 0", got)
	}

	a.SeriesTitle = stringPtr("幼女战记")
	a.SeriesYear = intPtr(2017)
	if got := a.MediaSeriesYear(); got != 2017 {
		t.Fatalf("mapped series year = %d", got)
	}
}

func TestMediaSeriesYearForLegacySequelWithoutSeasonField(t *testing.T) {
	a := &Anime{Title: "碧蓝之海 第三季", Year: intPtr(2026)}
	if got := a.MediaSeriesYear(); got != 0 {
		t.Fatalf("legacy sequel year = %d, want 0", got)
	}
}
