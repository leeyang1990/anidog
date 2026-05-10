package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	streamrulesvc "github.com/anidog/anidog-go/internal/service/streamrule"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupStreamRuleHandler() (*StreamRuleHandler, string) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(hash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	svc := streamrulesvc.NewService(db, nil)
	return NewStreamRuleHandler(svc), token
}

func TestStreamRule_List(t *testing.T) {
	h, token := setupStreamRuleHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/stream-rules", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestStreamRule_CreateAndGet(t *testing.T) {
	h, token := setupStreamRuleHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"name":                "test-rule",
		"base_url":            "https://example.com",
		"search_url":          "https://example.com/search",
		"search_list_xpath":   "//div",
		"search_name_xpath":   "//h3",
		"search_result_xpath": "//a",
		"chapter_result_xpath": "//div",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/stream-rules/", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body = %s", w.Code, w.Body.String())
	}

	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodGet, "/api/v1/stream-rules/"+uintToStr(id), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("get: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestStreamRule_Update(t *testing.T) {
	h, token := setupStreamRuleHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"name":                "old",
		"base_url":            "https://a.com",
		"search_url":          "https://a.com/search",
		"search_list_xpath":   "//div",
		"search_name_xpath":   "//h3",
		"search_result_xpath": "//a",
		"chapter_result_xpath": "//div",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/stream-rules/", body, token)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	updateBody := map[string]interface{}{
		"name": "new",
	}
	w = testutil.MakeRequest(router, http.MethodPut, "/api/v1/stream-rules/"+uintToStr(id), updateBody, token)
	if w.Code != http.StatusOK {
		t.Fatalf("update: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestStreamRule_Delete(t *testing.T) {
	h, token := setupStreamRuleHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"name":                "del",
		"base_url":            "https://a.com",
		"search_url":          "https://a.com/search",
		"search_list_xpath":   "//div",
		"search_name_xpath":   "//h3",
		"search_result_xpath": "//a",
		"chapter_result_xpath": "//div",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/stream-rules/", body, token)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	w = testutil.MakeRequest(router, http.MethodDelete, "/api/v1/stream-rules/"+uintToStr(id), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestStreamRule_Import(t *testing.T) {
	h, token := setupStreamRuleHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := []map[string]interface{}{
		{
			"name":          "kazumi-rule",
			"baseURL":       "https://kazumi.example.com",
			"searchURL":     "https://kazumi.example.com/search",
			"searchList":    "//div[@class='list']",
			"searchName":    "//h3",
			"searchResult":  "//a",
			"chapterResult": "//div[@class='ep']",
		},
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/stream-rules/import", body, token)
	if w.Code != http.StatusOK {
		t.Fatalf("import: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestStreamRule_Export(t *testing.T) {
	h, token := setupStreamRuleHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/stream-rules/export", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("export: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestStreamRule_InvalidID(t *testing.T) {
	h, token := setupStreamRuleHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/stream-rules/abc", nil, token)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

// ensure context import is used
var _ = context.Background
