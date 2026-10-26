package stream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/network"
)

func TestBrowserRecoveryFixtures(t *testing.T) {
	if os.Getenv("ANIDOG_BROWSER_FIXTURE") != "1" {
		t.Skip("browser fixture integration is opt-in")
	}
	provider := network.NewProxyProvider("")
	v := NewVideoInterceptor(&config.Config{RodHeadless: true, StreamInterceptTimeout: 30}, provider.Get)
	if err := v.Start(); err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	badServed := make(chan time.Time, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/catalog-456":
			w.WriteHeader(456)
			fmt.Fprint(w, "challenge")
		case "/no-referrer":
			http.SetCookie(w, &http.Cookie{Name: "media", Value: "allowed", Path: "/"})
			fmt.Fprint(w, `<script>fetch('/protected.m3u8', {referrerPolicy:'no-referrer'})</script>`)
		case "/protected.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, "#EXTM3U\n")
		case "/outer":
			fmt.Fprint(w, `<iframe src="/bad"></iframe>`)
		case "/bad":
			select {
			case badServed <- time.Now():
			default:
			}
			fmt.Fprint(w, `播放失败，请刷新或稍候再试！`)
		case "/ad-then-main":
			fmt.Fprint(w, `<script>fetch('/missing.m3u8').finally(()=>setTimeout(()=>fetch('/index.m3u8'),200))</script>`)
		case "/valid":
			fmt.Fprint(w, `<script>fetch('/index.m3u8')</script>`)
		case "/invalid":
			fmt.Fprint(w, `<script>fetch('/missing.m3u8')</script>`)
		case "/index.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, "#EXTM3U\n")
		case "/missing.m3u8":
			http.Error(w, "gone", http.StatusForbidden)
		}
	}))
	defer srv.Close()
	for _, path := range []string{"/outer"} {
		_, err := v.InterceptVideoURL(context.Background(), srv.URL+path, "", "")
		var player *playerFailure
		if !errors.As(err, &player) {
			t.Fatalf("%s: expected a player failure, got %v", path, err)
		}
		// Measure the detector after the fixture is actually served. Cold
		// browser-context/page creation is separately bounded by interception;
		// charging it to this short detector budget creates load-dependent
		// failures even when the error is correctly recognized immediately.
		select {
		case served := <-badServed:
			if elapsed := time.Since(served); elapsed > 8*time.Second {
				t.Fatalf("%s: player failure detection took %s after serving the error page", path, elapsed)
			}
		default:
			t.Fatal("interception failed without requesting the fixture error page")
		}
	}

	beforePages, _ := v.GetBrowser().Pages()
	_, err := v.interceptOnce(context.Background(), srv.URL+"/invalid", "", "", time.Second)
	var player *playerFailure
	if err == nil || errors.As(err, &player) {
		t.Fatalf("an HTTP error alone is retryable: %v", err)
	}
	afterPages, _ := v.GetBrowser().Pages()
	if len(afterPages) != len(beforePages) {
		t.Fatal("timed-out interception leaked a browser page")
	}
	video, err := v.InterceptVideoURL(context.Background(), srv.URL+"/valid", "", "")
	if err != nil || video.VideoType != "m3u8" {
		t.Fatalf("%v %v", video, err)
	}

	video, err = v.InterceptVideoURL(context.Background(), srv.URL+"/ad-then-main", "", "")
	if err != nil || video == nil || video.URL != srv.URL+"/index.m3u8" {
		t.Fatalf("auxiliary 403 hid the main media: %v %v", video, err)
	}
	video, err = v.InterceptVideoURL(context.Background(), srv.URL+"/no-referrer", "https://configured-referer.invalid/", "")
	if err != nil || video == nil || video.Referer != "" || video.UserAgent == "" || !strings.Contains(video.RequestHeaders().Get("Cookie"), "media=allowed") {
		t.Fatalf("browser request context was lost: referer=%v error=%v", video, err)
	}
	executor := NewStreamRuleExecutor(network.NewHTTPClient(&config.Config{}, provider), v)
	if _, err := executor.fetchWithBrowser(context.Background(), &model.StreamRule{ProxyMode: "direct"}, srv.URL+"/catalog-456", "", true); err == nil || !strings.Contains(err.Error(), "456") {
		t.Fatalf("browser challenge became successful empty catalog: %v", err)
	}
	// Each new proxy value gets its own context; no restart and no silent direct fallback.
	for _, label := range []string{"first-proxy", "second-proxy"} {
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "<body>%s</body>", label) }))
		provider.Set(proxy.URL)
		page, err := v.GetBrowser().Page(proto.TargetCreateTarget{URL: "about:blank"})
		if err != nil {
			t.Fatal(err)
		}
		err = page.Navigate("http://anidog-proxy-fixture.invalid/")
		if err != nil {
			t.Fatal(err)
		}
		body, err := page.Element("body")
		if err != nil {
			t.Fatal(err)
		}
		text, err := body.Text()
		if err != nil || text != label {
			t.Fatalf("proxy context not applied: %q %v", text, err)
		}
		_ = page.Close()
		proxy.Close()
	}
	direct := v.GetBrowserForRule(&model.StreamRule{ProxyMode: "direct"})
	if direct == nil || direct.BrowserContextID == v.GetBrowser().BrowserContextID {
		t.Fatal("direct rule reused the global proxy context")
	}
	video, err = v.InterceptVideoURL(context.Background(), srv.URL+"/valid", "", "", "direct")
	if err != nil || video.ProxyURL != "" {
		t.Fatalf("direct route did not reach media transport: %v %v", video, err)
	}
}
