package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service"
	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	"github.com/anidog/anidog-go/internal/testutil"
)

type fakeCalendarSource struct {
	snapshot service.CalendarSnapshot
	cleared  bool
}

func (f *fakeCalendarSource) ClearCache() { f.cleared = true }
func (f *fakeCalendarSource) GetCalendarSnapshot(context.Context) (service.CalendarSnapshot, error) {
	if len(f.snapshot.Days) == 0 {
		return f.snapshot, errors.New("offline")
	}
	return f.snapshot, nil
}

func TestCalendarLocalUsesThisWeekEpisodesNotLegacySunday(t *testing.T) {
	db := testutil.InitTestDB()
	sunday, count := 0, 12
	fixtures := []struct {
		title      string
		subscribed bool
		dates      []string
	}{
		{"Thursday", true, []string{"2026-10-08", "2026-10-08"}},
		{"Saturday", true, []string{"2026-10-10"}},
		{"Sunday", true, []string{"2026-10-11"}},
		{"Old finished", true, []string{"2007-04-08"}},
		{"Next week", true, []string{"2026-10-12"}},
		{"Last week", true, []string{"2026-10-04"}},
		{"Unknown", true, nil},
		{"Not subscribed", false, []string{"2026-10-08"}},
		{"Invalid date", true, []string{"2026-10-08-invalid"}},
		{"Monday", true, []string{"2026-10-05"}},
	}
	for _, f := range fixtures {
		a := model.Anime{Title: f.title, IsSubscribed: f.subscribed, AirWeekday: &sunday, EpisodeCount: &count}
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
		for i, date := range f.dates {
			ep := model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: i + 1, AirDate: &date}
			if err := db.Create(&ep).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	h := &CalendarHandler{animeSvc: animesvc.New(db), bangumiSvc: &fakeCalendarSource{}}
	router := testutil.SetupRouter()
	h.RegisterRoutes(router.Group("/api/v1"))
	for _, date := range []string{"2026-10-05", "2026-10-11"} {
		w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/calendar?metadata=1&date="+date, nil, "")
		var result struct {
			Days   []model.BangumiCalendarDayWithStatus `json:"days"`
			Source string                               `json:"source"`
			Start  string                               `json:"week_start"`
			End    string                               `json:"week_end"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatalf("response %d %s", w.Code, w.Body.String())
		}
		if result.Source != "local" || result.Start != "2026-10-05" || result.End != "2026-10-11" {
			t.Fatalf("wrong scope: %+v", result)
		}
		expected := map[int]string{0: "Sunday", 1: "Monday", 4: "Thursday", 6: "Saturday"}
		if len(result.Days) != 7 {
			t.Fatal("missing weekdays")
		}
		for _, day := range result.Days {
			want, exists := expected[day.WeekdayID]
			if !exists {
				if len(day.Items) > 0 {
					t.Fatalf("unexpected items: %+v", day)
				}
				continue
			}
			if len(day.Items) != 1 || day.Items[0].Name != want || !day.Items[0].IsSubscribed || day.Items[0].AirWeekday != day.WeekdayID {
				t.Fatalf("bad mapping: %+v", day)
			}
		}
	}
	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/calendar?date=2026-10-11", nil, "")
	var legacy []model.BangumiCalendarDayWithStatus
	if json.Unmarshal(w.Body.Bytes(), &legacy) != nil || len(legacy) != 7 {
		t.Fatal("legacy array contract broken")
	}
	w = testutil.MakeRequest(router, http.MethodGet, "/api/v1/calendar?date=not-a-date", nil, "")
	if w.Code != 400 {
		t.Fatal("invalid date accepted")
	}
}

func TestCalendarCachedStatusAndRefresh(t *testing.T) {
	db := testutil.InitTestDB()
	id := 7
	a := model.Anime{Title: "Subscribed", BangumiID: &id, IsSubscribed: true}
	db.Create(&a)
	source := &fakeCalendarSource{snapshot: service.CalendarSnapshot{Stale: true, UpdatedAt: time.Now(), Days: []service.BangumiCalendarDay{{WeekdayID: 0, Items: []service.BangumiAnime{{ID: 7, Name: "Subscribed"}}}}}}
	h := &CalendarHandler{animeSvc: animesvc.New(db), bangumiSvc: source}
	router := testutil.SetupRouter()
	h.RegisterRoutes(router.Group("/api/v1"))
	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/calendar?metadata=1", nil, "")
	var result struct {
		Source string                               `json:"source"`
		Days   []model.BangumiCalendarDayWithStatus `json:"days"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Source != "cache" || result.Days[0].Items[0].LocalID != a.ID {
		t.Fatal(w.Body.String())
	}
	w = testutil.MakeRequest(router, http.MethodPost, "/api/v1/calendar/refresh", nil, "")
	if w.Code != 200 || !source.cleared {
		t.Fatal("refresh not requested")
	}
}
