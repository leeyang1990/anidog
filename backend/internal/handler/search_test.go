package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/anidog/anidog-go/internal/service"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupSearchHandler() *SearchHandler {
	cfg := testutil.TestConfig()
	bangumiSvc := service.NewBangumiService(cfg)
	return NewSearchHandler(bangumiSvc)
}

func TestSearch_MissingKeyword(t *testing.T) {
	h := setupSearchHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/search", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestSearch_CollectSeason(t *testing.T) {
	h := setupSearchHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"anime_id": 1,
		"link":     "https://example.com/anime/1",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/search/collect-season", body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "整季收集功能开发中" {
		t.Errorf("expected stub message; got %v", resp["message"])
	}
}
