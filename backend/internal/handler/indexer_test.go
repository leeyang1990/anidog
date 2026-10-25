package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type indexerTransportFunc func(*http.Request) (*http.Response, error)

func (f indexerTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestManualIndexerSearchUsesApplicationHTTPClient(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	client := &http.Client{Transport: indexerTransportFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		seen[r.URL.Host]++
		mu.Unlock()
		body := "<html><body></body></html>"
		if strings.Contains(r.URL.Host, "bangumi.moe") {
			body = `{"torrents":[]}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	router := gin.New()
	NewIndexerHandler(nil, client).RegisterRoutes(router.Group("/api/v1"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/indexer/search", strings.NewReader(`{"keyword":"library recovery","indexers":["mikan","dmhy","bangumimoe","nyaa"]}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	for _, host := range []string{"mikanani.me", "www.dmhy.org", "bangumi.moe", "nyaa.si"} {
		if seen[host] == 0 {
			t.Fatalf("%s bypassed configured application transport: %v", host, seen)
		}
	}
}
