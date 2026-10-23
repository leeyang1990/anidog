package stream

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	episodeNameRe = regexp.MustCompile(`(?i)(?:^|[^0-9])(?:第\s*|EP(?:ISODE)?\s*)?(\d{1,4})(?:\.\d+)?(?:\s*[集话話])?(?:$|[^0-9])`)
	episodeURLRe  = regexp.MustCompile(`[-_/](\d{1,4})(?:\.html?)?(?:[?#].*)?$`)
)

// EpisodeNumber reads the provider's explicit episode label before its URL.
// Selection and execution use the same numbering convention.
func EpisodeNumber(item EpisodeInfo) (int, bool) {
	for _, candidate := range []struct {
		re *regexp.Regexp
		s  string
	}{{episodeNameRe, strings.TrimSpace(item.Name)}, {episodeURLRe, item.URL}} {
		match := candidate.re.FindStringSubmatch(candidate.s)
		if len(match) < 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// FindEpisode allows positional fallback only when the entire road has no
// explicit numbering, so descending or partially numbered lists stay safe.
func FindEpisode(items []EpisodeInfo, ep int) (EpisodeInfo, bool) {
	hasExplicit := false
	for _, item := range items {
		if n, ok := EpisodeNumber(item); ok {
			hasExplicit = true
			if n == ep {
				return item, true
			}
		}
	}
	if !hasExplicit && ep > 0 && ep <= len(items) {
		return items[ep-1], true
	}
	return EpisodeInfo{}, false
}

// EpisodeRoadsWithinCount excludes a whole road when its numbering exceeds
// the known subject boundary. A matching catalog title does not prove that a
// combined-season playlist uses the subscription's local episode numbering.
func EpisodeRoadsWithinCount(items []EpisodeInfo, count int) []EpisodeInfo {
	if count <= 0 {
		return items
	}
	blocked := make(map[string]bool)
	sizes := make(map[string]int)
	for _, item := range items {
		sizes[item.RoadName]++
		if n, ok := EpisodeNumber(item); ok && n > count {
			blocked[item.RoadName] = true
		}
	}
	var result []EpisodeInfo
	for _, item := range items {
		if !blocked[item.RoadName] && sizes[item.RoadName] <= count {
			result = append(result, item)
		}
	}
	return result
}
