package handler

import (
	"net/http"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	streamrulesvc "github.com/anidog/anidog-go/internal/service/streamrule"
	"github.com/anidog/anidog-go/internal/testutil"
	"github.com/anidog/anidog-go/internal/ws"
)

func TestStreamDownloadPersistsSourceIdentity(t *testing.T) {
	for _, batch := range []bool{false, true} {
		db := testutil.InitTestDB()
		a := model.Anime{Title: "Target"}
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
		rule := model.StreamRule{Name: "Source"}
		if err := db.Create(&rule).Error; err != nil {
			t.Fatal(err)
		}
		h := NewStreamHandler(streamrulesvc.NewService(db, nil), nil, dlservice.NewService(db, testutil.TestConfig(), nil))
		router := testutil.SetupRouter()
		h.RegisterRoutes(router.Group("/api/v1"))
		body := map[string]interface{}{"anime_id": a.ID, "anime_name": a.Title, "rule_id": rule.ID, "detail_url": "https://site/detail", "source_title": "Actual catalog title"}
		path := "/api/v1/stream/download"
		if batch {
			path += "/batch"
			body["episodes"] = []map[string]interface{}{{"url": "https://site/play-3", "number": 3, "road_name": "A"}}
		} else {
			body["episode_url"], body["episode_number"], body["road_name"] = "https://site/play-3", 3, "A"
		}
		w := testutil.MakeRequest(router, http.MethodPost, path, body, "")
		if w.Code != http.StatusOK {
			t.Fatalf("batch=%v status=%d body=%s", batch, w.Code, w.Body.String())
		}
		var d model.Download
		if err := db.First(&d).Error; err != nil {
			t.Fatal(err)
		}
		if d.AnimeID == nil || *d.AnimeID != a.ID || d.EpisodeNumber == nil || *d.EpisodeNumber != 3 || d.StreamDetailURL == nil || *d.StreamDetailURL != "https://site/detail" || d.StreamRoadName == nil || *d.StreamRoadName != "A" || d.SourceTitle != "Actual catalog title" {
			t.Fatalf("batch=%v lost identity: %+v", batch, d)
		}
	}
}

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
