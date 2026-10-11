package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
)

// Bangumi uses 1..7; AniDog uses 0..6. Missing/zero is unknown, not Sunday.
func resolveAirWeekday(raw *int, infobox []model.BangumiInfoKV, airDate string) int {
	if raw != nil && *raw >= 1 && *raw <= 7 {
		return *raw % 7
	}
	names := [][]string{
		{"星期日", "星期天", "周日", "週日", "日曜日", "Sunday"},
		{"星期一", "周一", "週一", "月曜日", "Monday"},
		{"星期二", "周二", "週二", "火曜日", "Tuesday"},
		{"星期三", "周三", "週三", "水曜日", "Wednesday"},
		{"星期四", "周四", "週四", "木曜日", "Thursday"},
		{"星期五", "周五", "週五", "金曜日", "Friday"},
		{"星期六", "周六", "週六", "土曜日", "Saturday"},
	}
	for _, kv := range infobox {
		if kv.Key != "放送星期" {
			continue
		}
		value := strings.Join(append([]string{kv.Value}, kv.Items...), " ")
		found := -1
		for day, aliases := range names {
			for _, alias := range aliases {
				if strings.Contains(value, alias) {
					if found >= 0 && found != day {
						return -1
					}
					found = day
				}
			}
		}
		if found >= 0 {
			return found
		}
	}
	if date, err := time.Parse("2006-01-02", airDate); err == nil {
		return int(date.Weekday())
	}
	return -1
}

type CalendarSnapshot struct {
	Days      []BangumiCalendarDay
	UpdatedAt time.Time
	Stale     bool
}

func (s *BangumiService) GetCalendar(ctx context.Context) ([]BangumiCalendarDay, error) {
	snapshot, err := s.GetCalendarSnapshot(ctx)
	return snapshot.Days, err
}

func (s *BangumiService) GetCalendarSnapshot(ctx context.Context) (CalendarSnapshot, error) {
	if cached, ok := s.getCached("calendar"); ok {
		return cached.(CalendarSnapshot), nil
	}
	// Briefly back off failed requests instead of delaying each page by five seconds.
	if _, failed := s.getCached("calendar:failed"); !failed {
		snapshot, err := s.fetchCalendar(ctx)
		if err == nil {
			s.setCache("calendar", snapshot, 6*time.Hour)
			s.setCache("calendar:last-good", snapshot, 7*24*time.Hour)
			return snapshot, nil
		}
		zap.L().Warn("Bangumi 放送日历请求失败", zap.Error(err))
		s.setCache("calendar:failed", true, time.Minute)
	}
	if cached, ok := s.getCached("calendar:last-good"); ok {
		snapshot := cached.(CalendarSnapshot)
		snapshot.Stale = true
		return snapshot, nil
	}
	return CalendarSnapshot{}, fmt.Errorf("放送日历源暂不可用，且没有有效缓存")
}

func (s *BangumiService) fetchCalendar(ctx context.Context) (CalendarSnapshot, error) {
	data, err := s.doRequest(ctx, http.MethodGet, "/calendar", nil)
	if err != nil {
		return CalendarSnapshot{}, err
	}
	var raw []struct {
		Weekday struct {
			ID int `json:"id"`
		} `json:"weekday"`
		Items []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			NameCN  string `json:"name_cn"`
			Summary string `json:"summary"`
			Images  struct {
				Common string `json:"common"`
			} `json:"images"`
			Rating struct {
				Score float64 `json:"score"`
			} `json:"rating"`
			AirDate  string `json:"air_date"`
			EpsCount int    `json:"eps_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CalendarSnapshot{}, err
	}
	days := make([]BangumiCalendarDay, 7)
	for i := range days {
		days[i] = BangumiCalendarDay{WeekdayID: i, WeekdayCN: weekdayCN[i], Items: []BangumiAnime{}}
	}
	seen := map[int]bool{}
	count := 0
	for _, day := range raw {
		if day.Weekday.ID < 1 || day.Weekday.ID > 7 || seen[day.Weekday.ID] {
			return CalendarSnapshot{}, fmt.Errorf("日历星期无效或重复")
		}
		seen[day.Weekday.ID] = true
		wd := day.Weekday.ID % 7
		for _, item := range day.Items {
			if item.ID <= 0 || (item.Name == "" && item.NameCN == "") {
				continue
			}
			days[wd].Items = append(days[wd].Items, BangumiAnime{ID: item.ID, Name: item.Name, NameCN: item.NameCN,
				Summary: item.Summary, ImageURL: item.Images.Common, Rating: item.Rating.Score,
				AirDate: item.AirDate, AirWeekday: wd, EpsCount: item.EpsCount})
			count++
		}
	}
	if len(seen) != 7 || count == 0 {
		return CalendarSnapshot{}, fmt.Errorf("日历数据不完整")
	}
	return CalendarSnapshot{Days: days, UpdatedAt: time.Now()}, nil
}
