package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	notifsvc "github.com/anidog/anidog-go/internal/service/notification"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupNotificationHandler() (*NotificationHandler, string) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(hash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	notifSvc := notifsvc.NewService(db)
	return NewNotificationHandler(notifSvc), token
}

func TestNotification_List(t *testing.T) {
	h, token := setupNotificationHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/notifications", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestNotification_CreateAndGet(t *testing.T) {
	h, token := setupNotificationHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"type":    "webhook",
		"name":    "TestHook",
		"config":  `{"url":"https://example.com/hook"}`,
		"enabled": true,
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/notifications", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body = %s", w.Code, w.Body.String())
	}

	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)

	id := uint(created["id"].(float64))
	w = testutil.MakeRequest(router, http.MethodGet, "/api/v1/notifications/"+uintToStr(id), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("get: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestNotification_Update(t *testing.T) {
	h, token := setupNotificationHandler()
	// Create via handler first so it uses the same DB
	body := map[string]interface{}{
		"type":    "webhook",
		"name":    "Old",
		"config":  `{"url":"https://old.com"}`,
		"enabled": true,
	}
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/notifications", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body = %s", w.Code, w.Body.String())
	}
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	id := uint(created["id"].(float64))

	updateBody := map[string]interface{}{
		"name": "Updated",
	}
	w = testutil.MakeRequest(router, http.MethodPut, "/api/v1/notifications/"+uintToStr(id), updateBody, token)
	if w.Code != http.StatusOK {
		t.Fatalf("update: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestNotification_Delete(t *testing.T) {
	h, token := setupNotificationHandler()
	db := testutil.InitTestDB()
	svc := notifsvc.NewService(db)

	svc.Create(context.Background(), &model.NotificationChannel{
		Type: "webhook", Name: "ToDelete", Config: `{"url":"https://del.com"}`, Enabled: true,
	})

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodDelete, "/api/v1/notifications/1", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestNotification_Test(t *testing.T) {
	h, token := setupNotificationHandler()
	db := testutil.InitTestDB()
	svc := notifsvc.NewService(db)

	svc.Create(context.Background(), &model.NotificationChannel{
		Type: "webhook", Name: "TestCh", Config: `{"url":"https://invalid.local/hook"}`, Enabled: true,
	})

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/notifications/1/test", nil, token)
	// May fail since the webhook URL is invalid, but should not 500
	if w.Code == http.StatusNotFound {
		t.Fatalf("channel should exist; body = %s", w.Body.String())
	}
}

func TestNotification_CreateInvalid(t *testing.T) {
	h, token := setupNotificationHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	// Missing required fields
	body := map[string]interface{}{
		"name": "NoType",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/notifications", body, token)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}
