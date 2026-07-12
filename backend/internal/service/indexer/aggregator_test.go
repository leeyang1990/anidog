package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/service/titleparse"
)

// mockIndexer 用于测试
type mockIndexer struct {
	name    string
	results []Candidate
	err     error
}

func (m *mockIndexer) Name() string { return m.name }
func (m *mockIndexer) Search(ctx context.Context, keyword string) ([]Candidate, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func TestAggregate_DeduplicationByInfoHash(t *testing.T) {
	a := &mockIndexer{name: "a", results: []Candidate{
		{Title: "[组A] 番 - 01 [1080p]", InfoHash: "ABC123", Size: 1000},
		{Title: "[组B] 番 - 01 [720p]", InfoHash: "DEF456", Size: 500},
	}}
	b := &mockIndexer{name: "b", results: []Candidate{
		{Title: "[组A] 番 - 01 [1080p]", InfoHash: "ABC123", Size: 1000}, // 重复
		{Title: "[组C] 番 - 02", InfoHash: "GHI789", Size: 800},
	}}

	got := Aggregate(context.Background(), []Indexer{a, b}, "番")
	if len(got) != 3 {
		t.Errorf("期望 3 条去重后结果，得到 %d: %+v", len(got), got)
	}

	// 验证每条都有 Parsed 字段
	for i, c := range got {
		if c.Parsed == nil {
			t.Errorf("第 %d 条没有 Parsed 字段", i)
		}
	}
}

func TestAggregate_SourceName(t *testing.T) {
	a := &mockIndexer{name: "test-source", results: []Candidate{
		{Title: "番 - 01", InfoHash: "X"},
	}}
	got := Aggregate(context.Background(), []Indexer{a}, "番")
	if len(got) != 1 {
		t.Fatal("expected 1 result")
	}
	if got[0].SourceName != "test-source" {
		t.Errorf("SourceName: got %q, want %q", got[0].SourceName, "test-source")
	}
}

func TestAggregate_IndexerErrorDoesNotBlock(t *testing.T) {
	a := &mockIndexer{name: "ok", results: []Candidate{
		{Title: "番 - 01", InfoHash: "X"},
	}}
	b := &mockIndexer{name: "fail", err: context.DeadlineExceeded}

	got := Aggregate(context.Background(), []Indexer{a, b}, "番")
	if len(got) != 1 {
		t.Errorf("期望 1 条（fail 应被跳过），得到 %d", len(got))
	}
}

func TestRankByPreference_EpisodeFilter(t *testing.T) {
	cands := []Candidate{
		{Title: "[LoliHouse] 番 - 01 [1080p][简繁内封]", InfoHash: "1"},
		{Title: "[LoliHouse] 番 - 02 [1080p][简繁内封]", InfoHash: "2"},
		{Title: "[LoliHouse] 番 - 03 [1080p][简繁内封]", InfoHash: "3"},
	}
	// 先让它们有 Parsed
	ctx := context.Background()
	_ = ctx
	idx := &mockIndexer{name: "x", results: cands}
	parsed := Aggregate(context.Background(), []Indexer{idx}, "番")

	ranked := RankByPreference(parsed, DownloadPreference{Quality: "1080p"}, 2)
	if len(ranked) != 1 {
		t.Fatalf("期望 1 条（集数 = 2），得到 %d", len(ranked))
	}
	if ranked[0].Parsed.EpisodeNum == nil || *ranked[0].Parsed.EpisodeNum != 2 {
		t.Errorf("应选中第 02 集")
	}
}

func TestRankByPreference_GroupWhitelistBoost(t *testing.T) {
	cands := []Candidate{
		{Title: "[未知组] 番 - 01 [1080p][简繁内封]", InfoHash: "1"},
		{Title: "[LoliHouse] 番 - 01 [1080p][简繁内封]", InfoHash: "2"},
	}
	idx := &mockIndexer{name: "x", results: cands}
	parsed := Aggregate(context.Background(), []Indexer{idx}, "番")

	ranked := RankByPreference(parsed, DownloadPreference{
		Quality: "1080p",
		Groups:  []string{"LoliHouse"},
	}, 1)
	if len(ranked) != 2 {
		t.Fatalf("期望 2 条，得到 %d", len(ranked))
	}
	// LoliHouse 应该在前
	if ranked[0].Parsed.Group != "LoliHouse" {
		t.Errorf("LoliHouse 应排第一，实际 %s", ranked[0].Parsed.Group)
	}
}

func TestRankByPreference_Batch(t *testing.T) {
	cands := []Candidate{
		{Title: "[桜都] 番 [01-12 Fin][1080P][简繁内封]", InfoHash: "X"},
	}
	idx := &mockIndexer{name: "x", results: cands}
	parsed := Aggregate(context.Background(), []Indexer{idx}, "番")

	// 集数 5 在批量包范围内 → 应该命中
	ranked := RankByPreference(parsed, DownloadPreference{}, 5)
	if len(ranked) != 1 {
		t.Errorf("批量包应命中 ep=5，实际得到 %d 条", len(ranked))
	}

	// 集数 20 超出批量包 → 不命中
	ranked2 := RankByPreference(parsed, DownloadPreference{}, 20)
	if len(ranked2) != 0 {
		t.Errorf("批量包不应命中 ep=20，实际得到 %d 条", len(ranked2))
	}
}

func TestCloseQuality(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1080p", "720p", true},
		{"1080p", "2160p", false},
		{"720p", "480p", true},
		{"", "1080p", false},
	}
	for _, tt := range tests {
		if got := closeQuality(tt.a, tt.b); got != tt.want {
			t.Errorf("closeQuality(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestScoreOne_NoPrefs(t *testing.T) {
	// 空 prefs 不应报错
	c := Candidate{
		Title:   "[LoliHouse] 番 - 01 [1080p]",
		Seeders: 10,
	}
	idx := &mockIndexer{name: "x", results: []Candidate{c}}
	parsed := Aggregate(context.Background(), []Indexer{idx}, "番")
	_ = time.Now() // silence unused

	ranked := RankByPreference(parsed, DownloadPreference{}, 1)
	if len(ranked) != 1 {
		t.Errorf("空偏好应保留所有条目")
	}
}

func TestRankByPreferenceAndSeasonRejectsExplicitMismatch(t *testing.T) {
	s2, s3, ep := 2, 3, 3
	cands := []Candidate{
		{Title: "wrong S2", Parsed: &titleparse.ParsedTitle{SeasonNum: &s2, EpisodeNum: &ep}},
		{Title: "right S3", Parsed: &titleparse.ParsedTitle{SeasonNum: &s3, EpisodeNum: &ep}},
		{Title: "unknown season", Parsed: &titleparse.ParsedTitle{EpisodeNum: &ep}},
	}
	got := RankByPreferenceAndSeason(cands, DownloadPreference{}, 3, 3)
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2", len(got))
	}
	for _, candidate := range got {
		if candidate.Title == "wrong S2" {
			t.Fatal("explicit season mismatch was not rejected")
		}
	}
}
