package episode

import (
	"github.com/anidog/anidog-go/internal/model"
	"testing"
	"time"
)

func TestSeasonFinishedRequiresPositiveEvidence(t *testing.T) {
	total := 12
	today := "2026-10-11"
	future := "2026-10-12"
	invalid := "unknown"
	now := time.Date(2026, 10, 11, 23, 0, 0, 0, time.FixedZone("CST", 8*3600))
	for _, tc := range []struct {
		name, status string
		count        *int
		eps          []model.AnimeEpisode
		want         bool
	}{
		{"unknown", model.AnimeStatusOngoing, &total, nil, false},
		{"finished", model.AnimeStatusFinished, &total, nil, true},
		{"aired finale", model.AnimeStatusOngoing, &total, []model.AnimeEpisode{{EpisodeNumber: 12, AirDate: &today}}, true},
		{"future beats status", model.AnimeStatusFinished, &total, []model.AnimeEpisode{{EpisodeNumber: 12, AirDate: &future}}, false},
		{"unknown count", model.AnimeStatusFinished, nil, nil, false},
		{"malformed date", model.AnimeStatusFinished, &total, []model.AnimeEpisode{{EpisodeNumber: 12, AirDate: &invalid}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := model.Anime{EpisodeCount: tc.count, Status: tc.status}
			if got := SeasonFinished(&a, tc.eps, now); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
