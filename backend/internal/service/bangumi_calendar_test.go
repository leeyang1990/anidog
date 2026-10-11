package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
)

func TestDetailWeekdayDoesNotDefaultToSunday(t *testing.T) {
	cases := []struct {
		name, fields string
		want         int
	}{
		{"missing", ``, -1},
		{"zero unknown", `,"air_weekday":0`, -1},
		{"null unknown", `,"air_weekday":null`, -1},
		{"legacy Sunday", `,"air_weekday":7`, 0},
		{"legacy Thursday", `,"air_weekday":4`, 4},
		{"v0 infobox", `,"date":"2026-10-01","infobox":[{"key":"放送星期","value":"星期六"}]`, 6},
		{"array infobox", `,"infobox":[{"key":"放送星期","value":[{"v":"水曜日"}]}]`, 3},
		{"date fallback", `,"date":"2026-10-01"`, 4},
		{"invalid date", `,"date":"2026-10"`, -1},
		{"ambiguous", `,"date":"2026-10-01","infobox":[{"key":"放送星期","value":"星期一、星期三"}]`, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"id":1,"name":"test"`+tc.fields+`}`) }))
			defer srv.Close()
			svc := NewBangumiService(&config.Config{BangumiAPIURL: srv.URL}, srv.Client())
			detail, err := svc.GetAnimeDetail(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if detail.AirWeekday != tc.want {
				t.Fatalf("weekday=%d want=%d", detail.AirWeekday, tc.want)
			}
		})
	}
}

func TestCalendarRefreshKeepsLastVerifiedSnapshot(t *testing.T) {
	var calls atomic.Int32
	var failing atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failing.Load() {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `[`)
		for day := 1; day <= 7; day++ {
			if day > 1 {
				fmt.Fprint(w, `,`)
			}
			fmt.Fprintf(w, `{"weekday":{"id":%d},"items":[{"id":%d,"name":"test"}]}`, day, day)
		}
		fmt.Fprint(w, `]`)
	}))
	defer srv.Close()
	svc := NewBangumiService(&config.Config{BangumiAPIURL: srv.URL}, srv.Client())
	first, err := svc.GetCalendarSnapshot(context.Background())
	if err != nil || first.Stale || len(first.Days) != 7 {
		t.Fatalf("snapshot=%+v err=%v", first, err)
	}
	if first.Days[0].WeekdayCN != "周日" || first.Days[0].Items[0].ID != 7 || first.Days[0].Items[0].AirWeekday != 0 {
		t.Fatal("Sunday not normalized")
	}
	failing.Store(true)
	svc.ClearCache()
	stale, err := svc.GetCalendarSnapshot(context.Background())
	if err != nil || !stale.Stale || !stale.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("refresh lost snapshot: %+v %v", stale, err)
	}
	_, _ = svc.GetCalendarSnapshot(context.Background())
	if calls.Load() != 2 {
		t.Fatalf("failed request not backed off: %d", calls.Load())
	}
	svc.setCache("calendar:last-good", first, -time.Second)
	if _, err := svc.GetCalendarSnapshot(context.Background()); err == nil {
		t.Fatal("expired snapshot served")
	}
}

func TestCalendarRejectsMalformedOrEmptySuccess(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `{}`, `[{"weekday":{"id":0},"items":[{"id":1,"name":"bad"}]}]`, `[{"weekday":{"id":7},"items":[{"id":1,"name":"partial"}]}]`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer srv.Close()
			svc := NewBangumiService(&config.Config{BangumiAPIURL: srv.URL, EnableDefaultRules: true}, srv.Client())
			if _, err := svc.GetCalendarSnapshot(context.Background()); err == nil {
				t.Fatal("invalid calendar accepted")
			}
		})
	}
}
