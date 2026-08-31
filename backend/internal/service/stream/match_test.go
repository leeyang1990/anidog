package stream

import "testing"

func TestPickBestMatchRejectsSharedPrefixOnly(t *testing.T) {
	results := []SearchResult{{Name: "碧蓝航线", URL: "wrong"}}
	if got := PickBestMatch("碧蓝之海 第3季", results); got != nil {
		t.Fatalf("shared-prefix mismatch selected: %+v", got)
	}
}

func TestPickBestMatchAcceptsExactBaseWithoutSeasonSuffix(t *testing.T) {
	results := []SearchResult{{Name: "碧蓝之海", URL: "right"}}
	got := PickBestMatch("碧蓝之海 第3季", results)
	if got == nil || got.URL != "right" {
		t.Fatalf("exact base title rejected: %+v", got)
	}
}

func TestPickBestMatchSkipsWrongHigherPosition(t *testing.T) {
	results := []SearchResult{
		{Name: "碧蓝航线", URL: "wrong"},
		{Name: "碧蓝之海 第三季", URL: "right"},
	}
	got := PickBestMatch("碧蓝之海 第3季", results)
	if got == nil || got.URL != "right" {
		t.Fatalf("got %+v, want confident matching result", got)
	}
}
