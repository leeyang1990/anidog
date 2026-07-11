package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupAuthHandler() (*AuthHandler, *authsvc.Service) {
	db := testutil.InitTestDB()
	svc := authsvc.New(db, "test-secret", 60*1e9)
	return NewAuthHandler(svc), svc
}

func makeAuthRequest(router *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestLogin_Success(t *testing.T) {
	h, authSvc := setupAuthHandler()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	authSvc.CreateUser(context.Background(), "testuser", "test@test.com", string(hash), false, true)

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	// Login uses form data
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("username=testuser&password=pass123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["access_token"] == nil {
		t.Error("response should contain access_token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h, authSvc := setupAuthHandler()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	authSvc.CreateUser(context.Background(), "testuser", "test@test.com", string(hash), false, true)

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("username=testuser&password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", w.Code)
	}
}

func TestLogin_DisabledUser(t *testing.T) {
	db := testutil.InitTestDB()
	svc := authsvc.New(db, "test-secret", 60*1e9)
	h := NewAuthHandler(svc)
	// Use raw SQL because GORM ignores IsActive=false (bool zero value → uses DB default true)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, email, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?, ?)",
		"disabled", "d@test.com", string(hash), false, false)

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("username=disabled&password=pass123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestLogin_EmptyCredentials(t *testing.T) {
	h, _ := setupAuthHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("username=&password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d; want 422", w.Code)
	}
}

func TestRegister_FirstUserBecomesAdmin(t *testing.T) {
	h, _ := setupAuthHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"username": "admin",
		"password": "admin123",
		"email":    "admin@test.com",
	}
	w := makeAuthRequest(router, http.MethodPost, "/api/v1/auth/register", body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["is_admin"] != true {
		t.Error("first user should be admin")
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	h, authSvc := setupAuthHandler()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	authSvc.CreateUser(context.Background(), "taken", "t@test.com", string(hash), true, true)

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"username": "taken",
		"password": "another123",
	}
	w := makeAuthRequest(router, http.MethodPost, "/api/v1/auth/register", body)

	// 400 = username taken; 403 = admin already exists and register blocked
	if w.Code != http.StatusBadRequest && w.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 400 or 403", w.Code)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	h, _ := setupAuthHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"username": "shortpw",
		"password": "12345",
	}
	w := makeAuthRequest(router, http.MethodPost, "/api/v1/auth/register", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestRefresh_WithValidToken(t *testing.T) {
	h, authSvc := setupAuthHandler()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	authSvc.CreateUser(context.Background(), "refreshuser", "r@test.com", string(hash), false, true)

	access, _, _ := authSvc.CreateTokenPair("refreshuser")

	// Need auth middleware to set username in context
	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		// Simulate auth middleware setting username
		c.Set("username", "refreshuser")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}
}
