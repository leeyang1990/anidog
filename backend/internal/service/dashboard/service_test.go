package dashboard

import (
	"context"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

type mockBangumiProvider struct {
	calendar []model.BangumiCalendarDay
	err      error
}

func (m *mockBangumiProvider) GetCalendar(_ context.Context) ([]model.BangumiCalendarDay, error) {
	return m.calendar, m.err
}

func setupDashboardSvc(provider BangumiProvider) *Service {
	db := testutil.InitTestDB()
	return New(db, provider)
}

func TestGetStats(t *testing.T) {
	svc := setupDashboardSvc(&mockBangumiProvider{})
	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.AnimeCount != 0 {
		t.Errorf("AnimeCount = %d; want 0", stats.AnimeCount)
	}
}

func TestGetStats_WithData(t *testing.T) {
	svc := setupDashboardSvc(&mockBangumiProvider{})
	svc.db.Create(&model.Anime{Title: "A", Status: model.AnimeStatusOngoing})
	svc.db.Create(&model.Download{Name: "dl1", TorrentID: "t1", URL: "http://x", Status: model.DownloadStatusCompleted})
	svc.db.Create(&model.Download{Name: "dl2", TorrentID: "t2", URL: "http://y", Status: model.DownloadStatusPending})

	stats, _ := svc.GetStats(context.Background())
	if stats.AnimeCount != 1 {
		t.Errorf("AnimeCount = %d; want 1", stats.AnimeCount)
	}
	if stats.DownloadStats.Total != 2 {
		t.Errorf("Total = %d; want 2", stats.DownloadStats.Total)
	}
	if stats.DownloadStats.Completed != 1 {
		t.Errorf("Completed = %d; want 1", stats.DownloadStats.Completed)
	}
}

func TestGetDownloadChart(t *testing.T) {
	svc := setupDashboardSvc(&mockBangumiProvider{})
	chart, err := svc.GetDownloadChart(context.Background())
	if err != nil {
		t.Fatalf("GetDownloadChart failed: %v", err)
	}
	if len(chart) != 7 {
		t.Errorf("chart len = %d; want 7", len(chart))
	}
}

func TestGetRecentDownloads(t *testing.T) {
	svc := setupDashboardSvc(&mockBangumiProvider{})
	svc.db.Create(&model.Download{Name: "dl1", TorrentID: "t1", URL: "http://x", Status: model.DownloadStatusCompleted})

	downloads, err := svc.GetRecentDownloads(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetRecentDownloads failed: %v", err)
	}
	if len(downloads) != 1 {
		t.Errorf("len = %d; want 1", len(downloads))
	}
}

func TestGetHotAnime(t *testing.T) {
	calendar := []model.BangumiCalendarDay{
		{
			WeekdayID: 1,
			WeekdayCN: "周一",
			Items: []model.BangumiAnime{
				{ID: 1, Name: "A", NameCN: "番A", Rating: 9.0},
				{ID: 2, Name: "B", NameCN: "番B", Rating: 7.5},
			},
		},
	}
	svc := setupDashboardSvc(&mockBangumiProvider{calendar: calendar})

	hot, err := svc.GetHotAnime(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetHotAnime failed: %v", err)
	}
	if len(hot) != 2 {
		t.Fatalf("len = %d; want 2", len(hot))
	}
	if hot[0].Rating < hot[1].Rating {
		t.Error("should be sorted by rating descending")
	}
}

func TestGetDashboard(t *testing.T) {
	svc := setupDashboardSvc(&mockBangumiProvider{})
	data, err := svc.GetDashboard(context.Background())
	if err != nil {
		t.Fatalf("GetDashboard failed: %v", err)
	}
	if data.Stats.AnimeCount != 0 {
		t.Errorf("unexpected stats")
	}
}
