package handler

import (
	"net/http"
	"testing"

	"github.com/anidog/anidog-go/internal/service/setting"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupSettingsHandler() (*SettingsHandler, string) {
	cfg := testutil.TestConfig()
	svc := setting.NewService(cfg)

	// Settings doesn't need auth, but for consistency
	return NewSettingsHandler(svc), ""
}

func TestSettings_GetSettings(t *testing.T) {
	h, _ := setupSettingsHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/settings", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestSettings_UpdateSettings(t *testing.T) {
	h, _ := setupSettingsHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"media_root": "/new/path",
	}
	w := testutil.MakeRequest(router, http.MethodPut, "/api/v1/settings", body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestSettings_GetSystemInfo(t *testing.T) {
	h, _ := setupSettingsHandler()
	router := testutil.SetupRouter()
	// System info is on /system group, not under /api/v1
	system := router.Group("/system")
	system.GET("/info", h.GetSystemInfo)

	w := testutil.MakeRequest(router, http.MethodGet, "/system/info", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}
