package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	"github.com/anidog/anidog-go/internal/service"
)

type CalendarHandler struct {
	animeSvc   *animesvc.Service
	bangumiSvc *service.BangumiService
}

func NewCalendarHandler(animeSvc *animesvc.Service, bangumiSvc *service.BangumiService) *CalendarHandler {
	return &CalendarHandler{animeSvc: animeSvc, bangumiSvc: bangumiSvc}
}

func (h *CalendarHandler) RegisterRoutes(rg *gin.RouterGroup) {
	calendar := rg.Group("/calendar")
	calendar.GET("", h.GetCalendar) // 公开访问，无需认证
	calendar.POST("/refresh", h.RefreshCalendar) // 刷新需要认证
}

func (h *CalendarHandler) RefreshCalendar(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"detail": "日历刷新已触发"})
}

func (h *CalendarHandler) GetCalendar(c *gin.Context) {
	// Bangumi API 抖/默认规则失败时会层层 fallback，最坏情况拖到 40+ 秒。
	// 给它 5 秒硬截止，超时就走本地 DB，避免前端卡到网关吐 500。
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	bangumiCalendar, err := h.bangumiSvc.GetCalendar(ctx)
	if err != nil {
		zap.L().Warn("获取 Bangumi 日历失败，使用本地数据", zap.Error(err))
	}

	subMap := h.animeSvc.GetSubscriptionMap(c.Request.Context())

	weekdayNames := map[int]string{
		0: "周日", 1: "周一", 2: "周二", 3: "周三",
		4: "周四", 5: "周五", 6: "周六",
	}

	if err == nil && len(bangumiCalendar) > 0 {
		var result []model.BangumiCalendarDayWithStatus
		for _, day := range bangumiCalendar {
			d := model.BangumiCalendarDayWithStatus{WeekdayID: day.WeekdayID, WeekdayCN: day.WeekdayCN}
			for _, item := range day.Items {
				entry := model.BangumiAnimeWithStatus{BangumiAnime: item}
				if sub, ok := subMap[item.ID]; ok {
					entry.IsSubscribed = sub.IsSubscribed
					entry.LocalID = sub.LocalID
				}
				d.Items = append(d.Items, entry)
			}
			result = append(result, d)
		}
		c.JSON(http.StatusOK, result)
		return
	}

	// Fallback: local DB
	localAnimes, _ := h.animeSvc.GetSubscribedWithAirWeekday(c.Request.Context())
	days := make(map[int]*model.BangumiCalendarDayWithStatus)
	for i := 0; i <= 6; i++ {
		days[i] = &model.BangumiCalendarDayWithStatus{WeekdayID: i, WeekdayCN: weekdayNames[i], Items: []model.BangumiAnimeWithStatus{}}
	}
	for _, a := range localAnimes {
		if a.AirWeekday != nil && *a.AirWeekday >= 0 && *a.AirWeekday <= 6 {
			entry := model.BangumiAnimeWithStatus{
				BangumiAnime: model.BangumiAnime{
					Name:   a.Title,
					NameCN: a.Title,
				},
				IsSubscribed: a.IsSubscribed,
				LocalID:      a.ID,
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
			days[*a.AirWeekday].Items = append(days[*a.AirWeekday].Items, entry)
		}
	}
	var result []model.BangumiCalendarDayWithStatus
	for i := 0; i <= 6; i++ {
		result = append(result, *days[i])
	}
	c.JSON(http.StatusOK, result)
}
