package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	dashboardsvc "github.com/anidog/anidog-go/internal/service/dashboard"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupDashboardHandler() (*DashboardHandler, string) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(hash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	dashSvc := dashboardsvc.New(db, &mockBangumiProvider{})
	return NewDashboardHandler(dashSvc), token
}

type mockBangumiProvider struct{}

func (m *mockBangumiProvider) GetCalendar(_ context.Context) ([]model.BangumiCalendarDay, error) {
	return nil, nil
}

func TestDashboard_GetStats(t *testing.T) {
	h, token := setupDashboardHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/dashboard/stats", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["anime_count"] == nil {
		t.Error("response should contain anime_count")
	}
}

func TestDashboard_GetDownloadChart(t *testing.T) {
	h, token := setupDashboardHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/dashboard/download-chart", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestDashboard_GetRecentDownloads(t *testing.T) {
	h, token := setupDashboardHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/dashboard/recent-downloads", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestDashboard_GetHotAnime(t *testing.T) {
	h, token := setupDashboardHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/dashboard/hot-anime", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestDashboard_GetDashboard(t *testing.T) {
	h, token := setupDashboardHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/dashboard", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

// ensure mockBangumiProvider satisfies the interface
var _ dashboardsvc.BangumiProvider = (*mockBangumiProvider)(nil)

// ensure animeSvc is imported (used by other tests in package)
var _ *animesvc.Service = nil
