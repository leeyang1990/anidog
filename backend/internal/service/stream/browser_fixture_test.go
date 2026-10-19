package stream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/network"
	"github.com/go-rod/rod/lib/proto"
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/outer":
			fmt.Fprint(w, `<iframe src="/bad"></iframe>`)
		case "/bad":
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
		start := time.Now()
		_, err := v.InterceptVideoURL(context.Background(), srv.URL+path, "", "")
		var player *playerFailure
		if !errors.As(err, &player) || time.Since(start) > 8*time.Second {
			t.Fatalf("%s: %v after %s", path, err, time.Since(start))
		}
	}

	_, err := v.interceptOnce(context.Background(), srv.URL+"/invalid", "", "", time.Second)
	var player *playerFailure
	if err == nil || errors.As(err, &player) {
		t.Fatalf("an HTTP error alone is retryable: %v", err)
	}
	video, err := v.InterceptVideoURL(context.Background(), srv.URL+"/valid", "", "")
	if err != nil || video.VideoType != "m3u8" {
		t.Fatalf("%v %v", video, err)
	}

	video, err = v.InterceptVideoURL(context.Background(), srv.URL+"/ad-then-main", "", "")
	if err != nil || video == nil || video.URL != srv.URL+"/index.m3u8" {
		t.Fatalf("auxiliary 403 hid the main media: %v %v", video, err)
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
