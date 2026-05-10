package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/testutil"
	"github.com/anidog/anidog-go/internal/ws"
)

func setupDownloadHandler() (*DownloadHandler, string) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(hash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	cfg := testutil.TestConfig()
	hub := ws.NewHub()
	dlSvc := dlservice.NewService(db, cfg, hub)
	return NewDownloadHandler(dlSvc), token
}

func TestDownload_List(t *testing.T) {
	h, token := setupDownloadHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/downloads", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["tasks"] == nil {
		t.Error("response should contain tasks")
	}
}

func TestDownload_CreateAndGet(t *testing.T) {
	h, token := setupDownloadHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"url":  "https://example.com/test.torrent",
		"name": "TestDownload",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/downloads/", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body = %s", w.Code, w.Body.String())
	}

	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodGet, "/api/v1/downloads/"+uintToStr(id), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("get: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestDownload_Delete(t *testing.T) {
	h, token := setupDownloadHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	// Create first
	body := map[string]interface{}{
		"url":  "https://example.com/del.torrent",
		"name": "ToDelete",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/downloads/", body, token)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodDelete, "/api/v1/downloads/"+uintToStr(id), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestDownload_Refresh(t *testing.T) {
	h, token := setupDownloadHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"url":  "https://example.com/refresh.torrent",
		"name": "ToRefresh",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/downloads/", body, token)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodPut, "/api/v1/downloads/"+uintToStr(id)+"/refresh", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestDownload_InvalidID(t *testing.T) {
	h, token := setupDownloadHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/downloads/abc", nil, token)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestDownload_PauseAll(t *testing.T) {
	h, token := setupDownloadHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/downloads/pause-all", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("pause-all: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestDownload_ResumeAll(t *testing.T) {
	h, token := setupDownloadHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/downloads/resume-all", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("resume-all: status = %d; body = %s", w.Code, w.Body.String())
	}
}

// ensure imports used
var _ *config.Config = nil
var _ *ws.Hub = nil
