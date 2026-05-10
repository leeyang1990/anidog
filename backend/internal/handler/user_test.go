package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	usersvc "github.com/anidog/anidog-go/internal/service/user"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupUserHandler() (*UserHandler, *authsvc.Service, string) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)
	userSvc := usersvc.New(db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(hash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	return NewUserHandler(userSvc, authSvc), authSvc, token
}

func TestUser_GetMe(t *testing.T) {
	h, _, token := setupUserHandler()
	router := testutil.SetupRouter()
	// Simulate auth middleware
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/users/me", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["username"] != "admin" {
		t.Errorf("username = %v; want admin", resp["username"])
	}
}

func TestUser_ChangePassword(t *testing.T) {
	h, _, token := setupUserHandler()
	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"old_password": "pass",
		"new_password": "newpass123",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/users/change-password", body, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestUser_ChangePassword_WrongOld(t *testing.T) {
	h, _, token := setupUserHandler()
	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"old_password": "wrongpass",
		"new_password": "newpass123",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/users/change-password", body, token)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", w.Code)
	}
}

func TestUser_ListUsers(t *testing.T) {
	h, _, token := setupUserHandler()
	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/users/", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestUser_CreateUser(t *testing.T) {
	h, _, token := setupUserHandler()
	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"username": "newuser",
		"password": "newpass123",
		"is_admin": false,
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/users/", body, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["username"] != "newuser" {
		t.Errorf("username = %v; want newuser", resp["username"])
	}
}

func TestUser_UpdateUser(t *testing.T) {
	h, authSvc, token := setupUserHandler()

	// Create a second user via the same authSvc (same DB)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	user2, _ := authSvc.CreateUser(nil, "to_update", "u@t.com", string(hash), false, true)

	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"is_active": false,
	}
	w := testutil.MakeRequest(router, http.MethodPut, "/api/v1/users/"+uintToStr(user2.ID), body, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestUser_DeleteUser(t *testing.T) {
	h, authSvc, token := setupUserHandler()

	// Create another user via same authSvc (same DB)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	user2, _ := authSvc.CreateUser(nil, "todelete", "t@t.com", string(hash), false, true)

	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodDelete, "/api/v1/users/"+uintToStr(user2.ID), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestUser_DeleteSelf(t *testing.T) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)
	userSvc := usersvc.New(db)

	// Insert admin with raw SQL to ensure GORM doesn't skip zero-value fields
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("adminpass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(adminHash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	// Get the admin's actual ID from DB
	var adminUser model.User
	db.Where("username = ?", "admin").First(&adminUser)

	h := NewUserHandler(userSvc, authSvc)
	router := testutil.SetupRouter()
	router.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodDelete, "/api/v1/users/"+uintToStr(adminUser.ID), nil, token)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400; body = %s", w.Code, w.Body.String())
	}
}
