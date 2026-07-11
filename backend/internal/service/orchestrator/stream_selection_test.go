package orchestrator

import (
	"testing"

	"github.com/anidog/anidog-go/internal/service/stream"
)

func TestSelectStreamEpisodeFallsBackToHealthyRoad(t *testing.T) {
	episodes := []stream.EpisodeInfo{
		{Name: "A1", URL: "a1", RoadName: "A"},
		{Name: "A2", URL: "a2", RoadName: "A"},
		{Name: "B1", URL: "b1", RoadName: "B"},
		{Name: "B2", URL: "b2", RoadName: "B"},
	}

	got, road, ok := selectStreamEpisode(episodes, 2, "A", map[string]bool{"A": true}, true)
	if !ok || road != "B" || got.URL != "b2" {
		t.Fatalf("expected B/b2 fallback, got road=%q url=%q ok=%v", road, got.URL, ok)
	}
}

func TestSelectStreamEpisodeHonorsSingleSourceRule(t *testing.T) {
	episodes := []stream.EpisodeInfo{
		{Name: "A1", URL: "a1", RoadName: "A"},
		{Name: "B1", URL: "b1", RoadName: "B"},
	}

	got, road, ok := selectStreamEpisode(episodes, 1, "A", map[string]bool{"A": true}, false)
	if !ok || road != "A" || got.URL != "a1" {
		t.Fatalf("expected configured road A, got road=%q url=%q ok=%v", road, got.URL, ok)
	}
}

func TestSelectStreamEpisodeSkipsRoadMissingEpisode(t *testing.T) {
	episodes := []stream.EpisodeInfo{
		{Name: "A1", URL: "a1", RoadName: "A"},
		{Name: "B1", URL: "b1", RoadName: "B"},
		{Name: "B2", URL: "b2", RoadName: "B"},
	}

	got, road, ok := selectStreamEpisode(episodes, 2, "A", nil, true)
	if !ok || road != "B" || got.URL != "b2" {
		t.Fatalf("expected B/b2, got road=%q url=%q ok=%v", road, got.URL, ok)
	}
}
