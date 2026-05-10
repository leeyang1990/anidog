package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/anidog/anidog-go/internal/config"
)

func TestAuthMiddleware_PublicPaths(t *testing.T) {
	cfg := &config.Config{SecretKey: "test-secret"}
	mw := AuthMiddleware(cfg)

	publicPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/ws/connect",
		"/healthcheck",
	}

	for _, path := range publicPaths {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			router.Use(mw)
			router.Any(path, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("path %s: status = %d; want 200", path, w.Code)
			}
		})
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	cfg := &config.Config{SecretKey: "test-secret"}
	mw := AuthMiddleware(cfg)

	router := gin.New()
	router.Use(mw)
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", w.Code)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	cfg := &config.Config{SecretKey: "test-secret"}
	mw := AuthMiddleware(cfg)

	router := gin.New()
	router.Use(mw)
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	cfg := &config.Config{SecretKey: "test-secret"}
	mw := AuthMiddleware(cfg)

	router := gin.New()
	router.Use(mw)
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret"
	cfg := &config.Config{SecretKey: secret}
	mw := AuthMiddleware(cfg)

	// Create a valid token
	claims := jwt.MapClaims{
		"sub": "testuser",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	var gotUsername string
	router := gin.New()
	router.Use(mw)
	router.GET("/api/v1/protected", func(c *gin.Context) {
		username, _ := c.Get("username")
		gotUsername, _ = username.(string)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}
	if gotUsername != "testuser" {
		t.Errorf("username = %q; want testuser", gotUsername)
	}
}

func TestIsPublicPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/auth/login", true},
		{"/api/v1/auth/register", true},
		{"/ws/connect", true},
		{"/healthcheck", true},
		{"/api/v1/anime", false},
		{"/api/v1/downloads", false},
		{"/api/v1/auth/refresh", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isPublicPath(tt.path); got != tt.want {
			t.Errorf("isPublicPath(%q) = %v; want %v", tt.path, got, tt.want)
		}
	}
}
