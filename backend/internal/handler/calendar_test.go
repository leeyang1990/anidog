package handler

import (
	"net/http"
	"testing"

	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	"github.com/anidog/anidog-go/internal/service"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupCalendarHandler() *CalendarHandler {
	db := testutil.InitTestDB()
	cfg := testutil.TestConfig()
	animeSvc := animesvc.New(db)
	bangumiSvc := service.NewBangumiService(cfg)
	return NewCalendarHandler(animeSvc, bangumiSvc)
}

func TestCalendar_GetCalendar(t *testing.T) {
	h := setupCalendarHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/calendar", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestCalendar_RefreshCalendar(t *testing.T) {
	h := setupCalendarHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/calendar/refresh", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}
