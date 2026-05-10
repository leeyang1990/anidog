package rss

import (
	"testing"

	"github.com/anidog/anidog-go/internal/model"
)

func TestMatch_NoRules(t *testing.T) {
	m := NewMatcher()
	if !m.Match("any title", nil) {
		t.Error("no rules should match everything")
	}
	if !m.Match("any title", []model.RSSRule{}) {
		t.Error("empty rules should match everything")
	}
}

func TestMatch_IncludeOnly(t *testing.T) {
	m := NewMatcher()
	rules := []model.RSSRule{
		{Keyword: "芙莉莲", Include: true, Enabled: true},
		{Keyword: "间谍", Include: true, Enabled: true},
	}

	if !m.Match("[ANi] 芙莉莲 第1集", rules) {
		t.Error("should match include keyword")
	}
	if m.Match("[ANi] 进击的巨人 第1集", rules) {
		t.Error("should not match when no include matches")
	}
}

func TestMatch_ExcludeOnly(t *testing.T) {
	m := NewMatcher()
	rules := []model.RSSRule{
		{Keyword: "HEVC", Include: false, Enabled: true},
	}

	if m.Match("[ANi] 芙莉莲 HEVC 第1集", rules) {
		t.Error("should be excluded by HEVC")
	}
	if !m.Match("[ANi] 芙莉莲 第1集", rules) {
		t.Error("should pass when no exclude matches")
	}
}

func TestMatch_IncludeAndExclude(t *testing.T) {
	m := NewMatcher()
	rules := []model.RSSRule{
		{Keyword: "芙莉莲", Include: true, Enabled: true},
		{Keyword: "HEVC", Include: false, Enabled: true},
	}

	if !m.Match("[ANi] 芙莉莲 第1集", rules) {
		t.Error("include match, no exclude → pass")
	}
	if m.Match("[ANi] 芙莉莲 HEVC 第1集", rules) {
		t.Error("exclude takes priority over include")
	}
	if m.Match("[ANi] 间谍过家家 第1集", rules) {
		t.Error("no include match → fail")
	}
}

func TestMatch_DisabledRule(t *testing.T) {
	m := NewMatcher()
	rules := []model.RSSRule{
		{Keyword: "芙莉莲", Include: true, Enabled: false},
	}
	if !m.Match("[ANi] 间谍过家家 第1集", rules) {
		t.Error("disabled rules are ignored; no active rules → match all")
	}
}

func TestMatch_RegexRule(t *testing.T) {
	m := NewMatcher()
	rules := []model.RSSRule{
		{Keyword: `^\[ANi\]`, Include: true, Enabled: true, IsRegex: true},
	}

	if !m.Match("[ANi] 芙莉莲 第1集", rules) {
		t.Error("regex should match")
	}
	if m.Match("[Moozzi2] 芙莉莲 第1集", rules) {
		t.Error("regex should not match")
	}
}

func TestMatch_InvalidRegex(t *testing.T) {
	m := NewMatcher()
	rules := []model.RSSRule{
		{Keyword: "[invalid", Include: true, Enabled: true, IsRegex: true},
	}
	// Invalid regex should not match (not crash)
	if m.Match("anything", rules) {
		t.Error("invalid regex should not match")
	}
}

func TestMatch_CaseInsensitive(t *testing.T) {
	m := NewMatcher()
	rules := []model.RSSRule{
		{Keyword: "hevc", Include: true, Enabled: true},
	}
	if !m.Match("[ANi] 芙莉莲 HEVC 第1集", rules) {
		t.Error("matching should be case-insensitive")
	}
}

func TestContainsMatch(t *testing.T) {
	tests := []struct {
		title, keyword string
		want           bool
	}{
		{"芙莉莲 第1集", "芙莉莲", true},
		{"Frieren EP1", "frieren", true},
		{"进击的巨人", "间谍", false},
		{"", "test", false},
		{"test", "", false},
	}
	for _, tt := range tests {
		got := containsMatch(tt.title, tt.keyword)
		if got != tt.want {
			t.Errorf("containsMatch(%q, %q) = %v; want %v", tt.title, tt.keyword, got, tt.want)
		}
	}
}
