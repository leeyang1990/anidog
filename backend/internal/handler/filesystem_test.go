package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newFileSystemTestRouter(root string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewFileSystemHandler(root).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func performFileSystemRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestFileSystemEndpointsRejectPathTraversal(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "downloads")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(outsideFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	router := newFileSystemTestRouter(root)
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "list", path: "/api/v1/filesystem/list", body: `{"path":"../outside"}`},
		{name: "create", path: "/api/v1/filesystem/create", body: `{"path":"../outside","dir_name":"created"}`},
		{name: "delete", path: "/api/v1/filesystem/delete", body: `{"path":"../outside","name":"keep.txt"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := performFileSystemRequest(router, http.MethodPost, tt.path, tt.body)
			if resp.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
			}
		})
	}

	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("outside file was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "created")); !os.IsNotExist(err) {
		t.Fatalf("directory was created outside root: %v", err)
	}
}

func TestFileSystemEndpointsRejectSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "downloads")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	router := newFileSystemTestRouter(root)
	resp := performFileSystemRequest(router, http.MethodPost, "/api/v1/filesystem/create", `{"path":"escape","dir_name":"created"}`)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if _, err := os.Stat(filepath.Join(outside, "created")); !os.IsNotExist(err) {
		t.Fatalf("directory was created through symlink: %v", err)
	}
}

func TestFileSystemEndpointsAllowRootOperationsButNotRootDeletion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	router := newFileSystemTestRouter(root)

	resp := performFileSystemRequest(router, http.MethodPost, "/api/v1/filesystem/create", `{"path":"/","dir_name":"anime"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if info, err := os.Stat(filepath.Join(root, "anime")); err != nil || !info.IsDir() {
		t.Fatalf("expected directory to be created: %v", err)
	}

	resp = performFileSystemRequest(router, http.MethodPost, "/api/v1/filesystem/delete", `{"path":"anime","name":".."}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("delete status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("download root was changed: %v", err)
	}
}
