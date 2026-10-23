package stream

// MatchingCatalogSource requires live catalog identity for a persisted detail
// URL. Selection may prefer this source; execution may only accept this source.
func MatchingCatalogSource(target, detailURL string, results []SearchResult) *SearchResult {
	for i := range results {
		if detailURL != "" && results[i].URL == detailURL && IsConfidentMatch(target, results[i].Name) {
			return &results[i]
		}
	}
	return nil
}

// EpisodeRoads preserves provider order and the common subject boundary.
// Both selection and execution must resolve numbers within a single road.
type EpisodeRoad struct {
	Name     string
	Episodes []EpisodeInfo
}

func EpisodeRoads(items []EpisodeInfo, count int) []EpisodeRoad {
	items = EpisodeRoadsWithinCount(items, count)
	var roads []EpisodeRoad
	indices := map[string]int{}
	for _, item := range items {
		i, exists := indices[item.RoadName]
		if !exists {
			i = len(roads)
			indices[item.RoadName] = i
			roads = append(roads, EpisodeRoad{Name: item.RoadName})
		}
		roads[i].Episodes = append(roads[i].Episodes, item)
	}
	return roads
}
