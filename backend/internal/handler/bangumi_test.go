package handler

import (
	"net/http"
	"testing"

	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	"github.com/anidog/anidog-go/internal/service"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupBangumiHandler() *BangumiHandler {
	db := testutil.InitTestDB()
	cfg := testutil.TestConfig()
	animeSvc := animesvc.New(db)
	bangumiSvc := service.NewBangumiService(cfg)
	return NewBangumiHandler(animeSvc, bangumiSvc, nil)
}

func TestBangumi_Search_MissingKeyword(t *testing.T) {
	h := setupBangumiHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/bangumi/search", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestBangumi_InvalidID(t *testing.T) {
	h := setupBangumiHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/bangumi/abc", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestBangumi_Calendar(t *testing.T) {
	h := setupBangumiHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/bangumi/calendar", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}
