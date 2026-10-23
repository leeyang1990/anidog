package stream

import "testing"

func TestEpisodeCountBoundaryAppliesToEachRoad(t *testing.T) {
	items := []EpisodeInfo{
		{Name: "1", URL: "full-1", RoadName: "combined"},
		{Name: "19", URL: "full-19", RoadName: "combined"},
		{Name: "8", URL: "arc-8", RoadName: "arc"},
		{Name: "1", URL: "arc-1", RoadName: "arc"},
	}
	filtered := EpisodeRoadsWithinCount(items, 8)
	if len(filtered) != 2 || filtered[0] != items[2] || filtered[1] != items[3] || len(items) != 4 {
		t.Fatal("count boundary retained combined season or mutated road order", filtered)
	}
	if ep, ok := FindEpisode(filtered, 1); !ok || ep.URL != "arc-1" {
		t.Fatal("descending list was treated as positional", ep, ok)
	}
	if len(EpisodeRoadsWithinCount(items, 0)) != len(items) {
		t.Fatal("unknown count discarded candidates")
	}
	named := []EpisodeInfo{{Name: "始まり", URL: "start"}, {Name: "帰還", URL: "return"}}
	if len(EpisodeRoadsWithinCount(named, 1)) != 0 {
		t.Fatal("oversized named playlist bypassed count boundary")
	}
}
