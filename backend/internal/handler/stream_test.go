package handler

import (
	"net/http"
	"testing"

	dlservice "github.com/anidog/anidog-go/internal/service/download"
	streamrulesvc "github.com/anidog/anidog-go/internal/service/streamrule"
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/testutil"
	"github.com/anidog/anidog-go/internal/ws"
)

func setupStreamHandler() *StreamHandler {
	db := testutil.InitTestDB()
	cfg := testutil.TestConfig()
	ruleSvc := streamrulesvc.NewService(db, nil)
	dlSvc := dlservice.NewService(db, cfg, ws.NewHub())
	return NewStreamHandler(ruleSvc, nil, dlSvc)
}

func TestStream_Search_MissingKeyword(t *testing.T) {
	h := setupStreamHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/stream/search", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestStream_Episodes_MissingParams(t *testing.T) {
	h := setupStreamHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/stream/episodes", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestStream_Download_MissingParams(t *testing.T) {
	h := setupStreamHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/stream/download", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestStream_BatchDownload_MissingParams(t *testing.T) {
	h := setupStreamHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/stream/download/batch", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestStream_CancelDownload_InvalidID(t *testing.T) {
	h := setupStreamHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPut, "/api/v1/stream/download/abc/cancel", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

// ensure imports used
var _ *config.Config = nil
