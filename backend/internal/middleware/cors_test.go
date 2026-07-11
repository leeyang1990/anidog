package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func corsTestRouter() *gin.Engine {
	router := gin.New()
	router.Use(CORS())
	router.POST("/api/v1/auth/register", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}

func TestCORSAllowsSameHost(t *testing.T) {
	router := corsTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://192.168.2.203:3002/api/v1/auth/register", nil)
	req.Header.Set("Origin", "http://192.168.2.203:3002")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
}

func TestCORSUsesForwardedHost(t *testing.T) {
	router := corsTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://backend:8088/api/v1/auth/register", nil)
	req.Host = "backend"
	req.Header.Set("Origin", "https://anime.example.com")
	req.Header.Set("X-Forwarded-Host", "anime.example.com")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
}

func TestCORSRejectsDifferentHost(t *testing.T) {
	router := corsTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://192.168.2.203:3002/api/v1/auth/register", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", w.Code)
	}
}
