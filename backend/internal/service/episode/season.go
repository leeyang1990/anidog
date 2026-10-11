package episode

import (
	"github.com/anidog/anidog-go/internal/model"
	"time"
)

// SeasonFinished requires a canonical episode count and positive airing evidence.
// A future date always wins over a stale finished flag or a misleading pack title.
func SeasonFinished(anime *model.Anime, episodes []model.AnimeEpisode, now time.Time) bool {
	if anime == nil || anime.EpisodeCount == nil || *anime.EpisodeCount <= 0 {
		return false
	}
	finalAired := false
	for _, ep := range episodes {
		if ep.EpisodeNumber < 1 || ep.EpisodeNumber > *anime.EpisodeCount || ep.AirDate == nil || *ep.AirDate == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", *ep.AirDate); err != nil {
			return false
		}
		if !IsAired(*ep.AirDate, now) {
			return false
		}
		if ep.EpisodeNumber == *anime.EpisodeCount {
			finalAired = true
		}
	}
	return finalAired || anime.Status == model.AnimeStatusFinished
}
