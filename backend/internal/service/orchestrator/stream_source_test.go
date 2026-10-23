package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
	"github.com/anidog/anidog-go/internal/testutil"
)

type fakeStreamCatalog struct {
	parsed   []uint
	combined bool
}

func TestDisabledStreamManagerRemainsNil(t *testing.T) {
	o := New(nil, nil, nil, nil, nil, t.TempDir())
	if o.streamMgr != nil {
		t.Fatal("disabled streaming became typed-nil catalog")
	}
}

func (f *fakeStreamCatalog) SearchAnime(_ context.Context, rule *model.StreamRule, title string) ([]stream.SearchResult, error) {
	return []stream.SearchResult{{Name: title, URL: fmt.Sprintf("detail-%d", rule.ID)}}, nil
}
func (f *fakeStreamCatalog) GetEpisodes(_ context.Context, rule *model.StreamRule, _ string) ([]stream.EpisodeInfo, error) {
	f.parsed = append(f.parsed, rule.ID)
	if f.combined {
		return []stream.EpisodeInfo{{Name: "1", URL: "combined-1", RoadName: "A"}, {Name: "19", URL: "combined-19", RoadName: "A"}}, nil
	}
	return []stream.EpisodeInfo{{Name: "第01集", URL: "a", RoadName: "A"}, {Name: "第01集", URL: "b", RoadName: "B"}}, nil
}

func TestMatchingCatalogTitleDoesNotValidateCombinedSeason(t *testing.T) {
	db := testutil.InitTestDB()
	if err := db.Create(&model.StreamRule{Name: "combined", Enabled: true, MultiSources: true}).Error; err != nil {
		t.Fatal(err)
	}
	count := 8
	a := model.Anime{Title: "Re：从零开始的异世界生活 第四季 夺还篇", EpisodeCount: &count}
	o := &Orchestrator{db: db, streamMgr: &fakeStreamCatalog{combined: true}}
	if _, err := o.resolveStreamSource(context.Background(), &a, 1); err == nil {
		t.Fatal("matching title admitted a playlist for a different episode range")
	}
}

func TestStreamSourceRecoveryPolicy(t *testing.T) {
	for _, scenario := range []string{"same site alternate road", "backup same road name", "all blocked", "expired rejection", "different detail", "legacy unknown road", "disabled current", "broken current", "half open current", "backup failure", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			db := testutil.InitTestDB()
			rules := []model.StreamRule{{Name: "current", Enabled: true, MultiSources: true}, {Name: "backup", Enabled: true, MultiSources: true}}
			for i := range rules {
				if err := db.Create(&rules[i]).Error; err != nil {
					t.Fatal(err)
				}
			}
			detail, preferred := fmt.Sprintf("detail-%d", rules[0].ID), "A"
			anime := model.Anime{Title: "example", StreamRuleID: &rules[0].ID, StreamDetailURL: &detail, StreamRoadName: &preferred}
			if err := db.Create(&anime).Error; err != nil {
				t.Fatal(err)
			}
			ep, now := 1, time.Now()
			addFailure := func(rule uint, page, road string, age time.Duration) {
				row := model.Download{TorrentID: fmt.Sprintf("%d-%s-%s", rule, page, road), Name: "failed", URL: "expired-token",
					AnimeID: &anime.ID, EpisodeNumber: &ep, Status: model.DownloadStatusFailed, DownloadType: model.DownloadTypeStream,
					FailureKind: model.FailureKindRejected, StreamRuleID: &rule, StreamDetailURL: &page, StreamRoadName: &road,
					UpdatedAt: now.Add(-age)}
				if err := db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			wantRule, wantRoad, wantError := rules[0].ID, "B", false
			switch scenario {
			case "same site alternate road":
				addFailure(rules[0].ID, detail, "A", 0)
			case "backup same road name", "all blocked", "backup failure":
				addFailure(rules[0].ID, detail, "A", 0)
				addFailure(rules[0].ID, detail, "B", 0)
				wantRule, wantRoad = rules[1].ID, "A"
				if scenario != "backup same road name" {
					addFailure(rules[1].ID, fmt.Sprintf("detail-%d", rules[1].ID), "A", 0)
					wantRoad = "B"
				}
				if scenario == "all blocked" {
					addFailure(rules[1].ID, fmt.Sprintf("detail-%d", rules[1].ID), "B", 0)
					wantError = true
				}
			case "expired rejection":
				addFailure(rules[0].ID, detail, "A", 25*time.Hour)
				wantRoad = "A"
			case "different detail":
				addFailure(rules[0].ID, "old-detail", "A", 0)
				wantRoad = "A"
			case "legacy unknown road":
				addFailure(rules[0].ID, "", "", 0)
				wantRule, wantRoad = rules[1].ID, "A"
			case "disabled current":
				if err := db.Model(&rules[0]).Update("enabled", false).Error; err != nil {
					t.Fatal(err)
				}
				wantRule, wantRoad = rules[1].ID, "A"
			case "broken current", "half open current":
				healthAt := now
				if scenario == "half open current" {
					healthAt = now.Add(-2 * time.Hour)
				}
				if err := db.Model(&rules[0]).Updates(map[string]interface{}{"health_status": "broken", "health_at": healthAt}).Error; err != nil {
					t.Fatal(err)
				}
				wantRule, wantRoad = rules[1].ID, "A"
				if scenario == "half open current" {
					wantRule = rules[0].ID
				}
			case "canceled":
				wantError = true
			}
			catalog := &fakeStreamCatalog{}
			o := &Orchestrator{db: db, streamMgr: catalog}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			choice, err := o.resolveStreamSource(ctx, &anime, ep)
			if wantError {
				if err == nil {
					t.Fatal("expected no eligible source")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if choice.rule.ID != wantRule || choice.episode.RoadName != wantRoad {
				t.Fatalf("got rule=%d road=%s, want %d/%s", choice.rule.ID, choice.episode.RoadName, wantRule, wantRoad)
			}
			if *anime.StreamRuleID != rules[0].ID {
				t.Fatal("search mutated preference before enqueue")
			}
		})
	}
}

func TestCandidateCooldownBoundary(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		kind  string
		retry int
		delay time.Duration
	}{
		{model.FailureKindTransient, 0, 10 * time.Minute}, {model.FailureKindTransient, 1, time.Hour},
		{model.FailureKindTransient, 2, 6 * time.Hour}, {model.FailureKindExhausted, 3, 24 * time.Hour},
		{model.FailureKindRejected, 0, 24 * time.Hour}, {model.FailureKindPermanent, 0, 6 * time.Hour},
	} {
		futurePoll := now.Add(7 * 24 * time.Hour)
		failure := model.Download{FailureKind: tc.kind, RetryCount: tc.retry, UpdatedAt: now, NextRetryAt: &futurePoll}
		if got := streamCandidateRetryAt(failure); !got.Equal(now.Add(tc.delay)) {
			t.Fatalf("%s retry=%d got=%v", tc.kind, tc.retry, got)
		}
	}
}
