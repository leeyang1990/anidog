package stream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/network"
)

func TestRuleHTTPRouteDoesNotChangeGlobalProxy(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "direct") }))
	defer origin.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "proxy") }))
	defer proxy.Close()
	provider := network.NewProxyProvider(proxy.URL)
	e := NewStreamRuleExecutor(network.NewHTTPClient(&config.Config{}, provider), nil)
	for _, mode := range []string{"inherit", "direct", "inherit"} {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", origin.URL, nil)
		r, err := e.clientForRule(&model.StreamRule{ProxyMode: mode}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		want := "proxy"
		if mode == "direct" {
			want = "direct"
		}
		if string(body) != want {
			t.Fatalf("%s reached %s", mode, body)
		}
	}
	if provider.Get() != proxy.URL {
		t.Fatal("per-rule routing changed the global proxy")
	}
}
