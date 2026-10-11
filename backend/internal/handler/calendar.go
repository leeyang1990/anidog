package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service"
	animesvc "github.com/anidog/anidog-go/internal/service/anime"
)

type calendarSource interface {
	GetCalendarSnapshot(context.Context) (service.CalendarSnapshot, error)
	ClearCache()
}

type CalendarHandler struct {
	animeSvc   *animesvc.Service
	bangumiSvc calendarSource
}

func NewCalendarHandler(animeSvc *animesvc.Service, bangumiSvc *service.BangumiService) *CalendarHandler {
	return &CalendarHandler{animeSvc: animeSvc, bangumiSvc: bangumiSvc}
}

func (h *CalendarHandler) RegisterRoutes(rg *gin.RouterGroup) {
	calendar := rg.Group("/calendar")
	calendar.GET("", h.GetCalendar)
	calendar.POST("/refresh", h.RefreshCalendar)
}

func (h *CalendarHandler) RefreshCalendar(c *gin.Context) {
	h.bangumiSvc.ClearCache()
	c.JSON(http.StatusOK, gin.H{"detail": "日历刷新已触发"})
}

func (h *CalendarHandler) GetCalendar(c *gin.Context) {
	now := time.Now()
	if date := c.Query("date"); date != "" {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "日期格式应为 YYYY-MM-DD"})
			return
		}
		now = parsed
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(int(now.Weekday())+6)%7)
	end := start.AddDate(0, 0, 7)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	snapshot, err := h.bangumiSvc.GetCalendarSnapshot(ctx)
	days := make([]model.BangumiCalendarDayWithStatus, 7)
	names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	for i := range days {
		days[i] = model.BangumiCalendarDayWithStatus{WeekdayID: i, WeekdayCN: names[i], Items: []model.BangumiAnimeWithStatus{}}
	}
	source := "bangumi"
	if err == nil && len(snapshot.Days) > 0 {
		if snapshot.Stale {
			source = "cache"
		}
		subMap := h.animeSvc.GetSubscriptionMap(c.Request.Context())
		for _, day := range snapshot.Days {
			if day.WeekdayID < 0 || day.WeekdayID > 6 {
				continue
			}
			for _, item := range day.Items {
				entry := model.BangumiAnimeWithStatus{BangumiAnime: item}
				if sub, ok := subMap[item.ID]; ok {
					entry.IsSubscribed = sub.IsSubscribed
					entry.LocalID = sub.LocalID
				}
				days[day.WeekdayID].Items = append(days[day.WeekdayID].Items, entry)
			}
		}
	} else {
		zap.L().Warn("获取放送日历失败，按本周单集排期降级", zap.Error(err))
		source = "local"
		local, dbErr := h.animeSvc.GetSubscribedForCalendar(c.Request.Context(), start.Format("2006-01-02"), end.Format("2006-01-02"))
		if dbErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "日历源与本地排期均不可用"})
			return
		}
		for _, a := range local {
			seen := map[int]bool{}
			for _, ep := range a.Episodes {
				if a.EpisodeCount != nil && *a.EpisodeCount > 0 && ep.EpisodeNumber > *a.EpisodeCount {
					continue
				}
				if ep.AirDate == nil {
					continue
				}
				date, parseErr := time.ParseInLocation("2006-01-02", *ep.AirDate, start.Location())
				if parseErr != nil || date.Before(start) || !date.Before(end) {
					continue
				}
				wd := int(date.Weekday())
				if seen[wd] {
					continue
				}
				seen[wd] = true
				entry := model.BangumiAnimeWithStatus{
					BangumiAnime: model.BangumiAnime{Name: a.Title, NameCN: a.Title, AirDate: *ep.AirDate, AirWeekday: wd},
					IsSubscribed: true, LocalID: a.ID,
				}
				if a.CoverURL != nil {
					entry.ImageURL = *a.CoverURL
				}
				if a.BangumiRating != nil {
					entry.Rating = *a.BangumiRating
				}
				if a.BangumiID != nil {
					entry.ID = *a.BangumiID
				}
				days[wd].Items = append(days[wd].Items, entry)
			}
		}
	}
	if c.Query("metadata") == "1" {
		c.JSON(http.StatusOK, gin.H{"days": days, "source": source, "updated_at": snapshot.UpdatedAt,
			"week_start": start.Format("2006-01-02"), "week_end": end.AddDate(0, 0, -1).Format("2006-01-02")})
	} else {
		// Keep the original array contract for existing clients.
		c.JSON(http.StatusOK, days)
	}
}
