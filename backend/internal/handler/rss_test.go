package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	rsssvc "github.com/anidog/anidog-go/internal/service/rss"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupRSSHandler() (*RSSHandler, string) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(hash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	crudSvc := rsssvc.NewCRUDService(db)
	return NewRSSHandler(crudSvc, nil), token // nil engine for CRUD tests
}

func TestRSS_List(t *testing.T) {
	h, token := setupRSSHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/rss", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRSS_CreateAndGet(t *testing.T) {
	h, token := setupRSSHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"name":    "Mikan",
		"url":     "https://mikan.example.com/rss",
		"enabled": true,
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/rss/", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body = %s", w.Code, w.Body.String())
	}

	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodGet, "/api/v1/rss/"+uintToStr(id), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("get: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRSS_Update(t *testing.T) {
	h, token := setupRSSHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	// Create first
	body := map[string]interface{}{
		"name":    "Old",
		"url":     "https://old.com/rss",
		"enabled": true,
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/rss/", body, token)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	updateBody := map[string]interface{}{
		"name": "New",
	}
	w = testutil.MakeRequest(router, http.MethodPut, "/api/v1/rss/"+uintToStr(id), updateBody, token)
	if w.Code != http.StatusOK {
		t.Fatalf("update: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRSS_Delete(t *testing.T) {
	h, token := setupRSSHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"name":    "Del",
		"url":     "https://del.com/rss",
		"enabled": true,
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/rss/", body, token)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodDelete, "/api/v1/rss/"+uintToStr(id), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRSS_RuleCRUD(t *testing.T) {
	h, token := setupRSSHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	// Create feed
	feedBody := map[string]interface{}{
		"name":    "Feed",
		"url":     "https://feed.com/rss",
		"enabled": true,
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/rss/", feedBody, token)
	var feed map[string]interface{}
	json.NewDecoder(w.Body).Decode(&feed)
	feedID := uint(feed["id"].(float64))

	// Create rule
	ruleBody := map[string]interface{}{
		"name":    "Rule1",
		"keyword": "芙莉莲",
		"include": true,
		"enabled": true,
	}
	w = testutil.MakeRequest(router, http.MethodPost, "/api/v1/rss/"+uintToStr(feedID)+"/rules", ruleBody, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create rule: status = %d; body = %s", w.Code, w.Body.String())
	}

	// List rules
	w = testutil.MakeRequest(router, http.MethodGet, "/api/v1/rss/"+uintToStr(feedID)+"/rules", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("list rules: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRSS_ManualCheck(t *testing.T) {
	h, token := setupRSSHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	// Create feed first
	body := map[string]interface{}{
		"name":    "CheckFeed",
		"url":     "https://check.com/rss",
		"enabled": true,
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/rss/", body, token)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodPost, "/api/v1/rss/"+uintToStr(id)+"/check", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("check: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRSS_GetItems(t *testing.T) {
	h, token := setupRSSHandler()
	db := testutil.InitTestDB()
	crudSvc := rsssvc.NewCRUDService(db)
	feed := &model.RSSFeed{Name: "ItemsFeed", URL: "https://items.com/rss", Enabled: true}
	crudSvc.CreateFeed(context.Background(), feed)

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/rss/"+uintToStr(feed.ID)+"/items", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("items: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRSS_InvalidID(t *testing.T) {
	h, token := setupRSSHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/rss/abc", nil, token)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}
