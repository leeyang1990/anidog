package stream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/service/network"
)

func TestMediaRouteRecoveryIsBoundedAndKeepsQualityPolicy(t *testing.T) {
	for _, scenario := range []string{"recover", "both failed", "quality rejection", "explicit direct", "tool unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			m := &StreamManager{preflightSem: make(chan struct{}, 1)}
			video := &InterceptedVideo{URL: "https://cdn.invalid/video.mp4", ProxyURL: "http://proxy.invalid:8080", Headers: `{"User-Agent":["browser-agent"]}`}
			rule := &model.StreamRule{ProxyMode: "inherit"}
			if scenario == "explicit direct" {
				rule.ProxyMode = "direct"
				video.ProxyURL = ""
			}
			calls := 0
			ctx, closeRoute, err := m.preflightVideo(context.Background(), video, rule, []func(context.Context, *InterceptedVideo) error{func(ctx context.Context, got *InterceptedVideo) error {
				calls++
				if got.RequestHeaders().Get("User-Agent") != "browser-agent" {
					t.Fatal("headers changed on route retry")
				}
				if scenario == "quality rejection" {
					return errors.New("流媒体预检不符合要求：1280x720")
				}
				if scenario == "tool unavailable" {
					return errors.New("ffprobe not installed")
				}
				if calls == 1 || scenario == "both failed" {
					return &mediacheck.InputError{Err: errors.New("HTTP 403")}
				}
				return nil
			}})
			defer closeRoute()
			if ctx == nil || (err == nil) != (scenario == "recover") {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			want := 1
			if scenario == "recover" || scenario == "both failed" {
				want = 2
			}
			if calls != want {
				t.Fatalf("calls=%d want=%d", calls, want)
			}
			if scenario == "recover" && video.ProxyURL != "" {
				t.Fatal("winning route not frozen")
			}
		})
	}
}

func TestCatalogHTTPFailureCannotBecomeEmptySearchSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(456); fmt.Fprint(w, "challenge") }))
	defer srv.Close()
	executor := NewStreamRuleExecutor(network.NewHTTPClient(&config.Config{}, network.NewProxyProvider("")), nil)
	if _, err := executor.fetchWithHTTP(context.Background(), &model.StreamRule{ProxyMode: "direct"}, srv.URL, "show"); err == nil {
		t.Fatal("HTTP challenge silently parsed as an empty catalog")
	}
}
