package dashboard

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

// BangumiProvider is a narrow interface for calendar data.
type BangumiProvider interface {
	GetCalendar(ctx context.Context) ([]model.BangumiCalendarDay, error)
}

type Service struct {
	db         *gorm.DB
	bangumiSvc BangumiProvider
}

func New(db *gorm.DB, bangumiSvc BangumiProvider) *Service {
	return &Service{db: db, bangumiSvc: bangumiSvc}
}

// Stats holds dashboard statistics.
type Stats struct {
	AnimeCount    int64         `json:"anime_count"`
	RSSFeedCount  int64         `json:"rss_feed_count"`
	DownloadStats DownloadStats `json:"download_stats"`
}

// DownloadStats holds download status counts.
type DownloadStats struct {
	Total       int64 `json:"total"`
	Pending     int64 `json:"pending"`
	Downloading int64 `json:"downloading"`
	Completed   int64 `json:"completed"`
	Failed      int64 `json:"failed"`
	Paused      int64 `json:"paused"`
}

func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	var animeCount int64
	s.db.WithContext(ctx).Model(&model.Anime{}).Count(&animeCount)

	var rssFeedCount int64
	s.db.WithContext(ctx).Model(&model.RSSFeed{}).Count(&rssFeedCount)

	ds := DownloadStats{}
	s.db.WithContext(ctx).Model(&model.Download{}).Count(&ds.Total)

	statusCounts := []struct {
		Status string
		Count  *int64
	}{
		{model.DownloadStatusPending, &ds.Pending},
		{model.DownloadStatusDownloading, &ds.Downloading},
		{model.DownloadStatusCompleted, &ds.Completed},
		{model.DownloadStatusFailed, &ds.Failed},
		{model.DownloadStatusPaused, &ds.Paused},
	}
	for _, sc := range statusCounts {
		s.db.WithContext(ctx).Model(&model.Download{}).Where("status = ?", sc.Status).Count(sc.Count)
	}

	return &Stats{
		AnimeCount:    animeCount,
		RSSFeedCount:  rssFeedCount,
		DownloadStats: ds,
	}, nil
}

// ChartDataPoint is a single data point for the download chart.
type ChartDataPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func (s *Service) GetDownloadChart(ctx context.Context) ([]ChartDataPoint, error) {
	now := time.Now()
	dataPoints := make([]ChartDataPoint, 7)
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dataPoints[6-i].Date = date.Format("2006-01-02")
	}
	for i := range dataPoints {
		dayStart, _ := time.Parse("2006-01-02", dataPoints[i].Date)
		dayEnd := dayStart.Add(24 * time.Hour)
		var count int64
		s.db.WithContext(ctx).Model(&model.Download{}).Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).Count(&count)
		dataPoints[i].Count = count
	}
	return dataPoints, nil
}

func (s *Service) GetRecentDownloads(ctx context.Context, limit int) ([]model.Download, error) {
	var downloads []model.Download
	err := s.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&downloads).Error
	return downloads, err
}

func (s *Service) GetHotAnime(ctx context.Context, limit int) ([]model.BangumiAnimeWithStatus, error) {
	calendar, err := s.bangumiSvc.GetCalendar(ctx)
	if err != nil {
		return nil, err
	}

	subMap := make(map[int]model.SubStatus)
	var subscribed []model.Anime
	s.db.WithContext(ctx).Where("bangumi_id IS NOT NULL").Find(&subscribed)
	for _, a := range subscribed {
		if a.BangumiID != nil {
			subMap[*a.BangumiID] = model.SubStatus{
				IsSubscribed: a.IsSubscribed,
				LocalID:      a.ID,
			}
		}
	}

	var allAnime []model.BangumiAnimeWithStatus
	for _, day := range calendar {
		for _, item := range day.Items {
			entry := model.BangumiAnimeWithStatus{BangumiAnime: item}
			if sub, ok := subMap[item.ID]; ok {
				entry.IsSubscribed = sub.IsSubscribed
				entry.LocalID = sub.LocalID
			}
			allAnime = append(allAnime, entry)
		}
	}

	sort.Slice(allAnime, func(i, j int) bool {
		return allAnime[i].Rating > allAnime[j].Rating
	})

	if len(allAnime) > limit {
		allAnime = allAnime[:limit]
	}
	return allAnime, nil
}

// DashboardData is the combined dashboard response.
type DashboardData struct {
	Stats           Stats            `json:"stats"`
	DownloadChart   []ChartDataPoint `json:"downloadStats"`
	RecentDownloads []model.Download `json:"recentDownloads"`
}

func (s *Service) GetDashboard(ctx context.Context) (*DashboardData, error) {
	stats, _ := s.GetStats(ctx)
	chart, _ := s.GetDownloadChart(ctx)
	recent, _ := s.GetRecentDownloads(ctx, 5)

	if stats == nil {
		stats = &Stats{}
	}
	if chart == nil {
		chart = []ChartDataPoint{}
	}
	if recent == nil {
		recent = []model.Download{}
	}

	return &DashboardData{
		Stats:           *stats,
		DownloadChart:   chart,
		RecentDownloads: recent,
	}, nil
}
