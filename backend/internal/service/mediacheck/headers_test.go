package mediacheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserHeadersReachProbeFrameAndTransferWithoutInventedReferer(t *testing.T) {
	path := transportVideo(t, "mp4")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, cookieErr := r.Cookie("media")
		if r.Header.Get("Referer") != "" || r.UserAgent() != "browser-agent" || r.Header.Get("Origin") != "https://player.invalid" || cookieErr != nil || cookie.Value != "allowed" {
			t.Logf("fixture ua=%q origin=%q cookie=%q ref=%q", r.UserAgent(), r.Header.Get("Origin"), r.Header.Get("Cookie"), r.Referer())
			http.Error(w, "request context changed", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, path)
	}))
	defer server.Close()
	headers := http.Header{"User-Agent": {"browser-agent"}, "Origin": {"https://player.invalid"}, "Cookie": {"media=allowed"}}
	ctx := WithHTTPProxy(context.Background(), "")
	if _, err := ProbeInput(ctx, server.URL+"/video.mp4", MediaRequestArgs("ffprobe", server.URL+"/video.mp4", "mp4", headers)); err != nil {
		t.Fatal(err)
	}
	for _, output := range [][]string{{"-frames:v", "1", "-f", "null", "-"}, {"-c", "copy", "-f", "null", "-"}} {
		args := append([]string{"-v", "error"}, MediaRequestArgs("ffmpeg", server.URL+"/video.mp4", "mp4", headers)...)
		args = append(args, "-i", server.URL+"/video.mp4")
		if out, err := MediaCommand(ctx, "ffmpeg", append(args, output...)...).CombinedOutput(); err != nil {
			t.Fatalf("frame/transfer: %v %s", err, out)
		}
	}
}

func TestRequestHeadersRejectInjectionAndBrowserRange(t *testing.T) {
	headers := http.Header{"User-Agent": {"ok\r\nInjected: bad"}, "Referer": {"https://player.invalid/"}, "Range": {"bytes=0-4095"}, "Host": {"wrong"}, "Proxy-Authorization": {"secret"}, "If-None-Match": {"stale"}}
	args := strings.Join(MediaRequestArgs("ffmpeg", "https://cdn.invalid/video.mp4", "mp4", headers), " ")
	if strings.Contains(args, "bad") || strings.Contains(args, "secret") || strings.Contains(args, "4095") || strings.Contains(args, "stale") || !strings.Contains(args, "https://player.invalid/") {
		t.Fatal(args)
	}
}

func TestMediaCookiesStayOnOriginalHostAcrossRedirects(t *testing.T) {
	path := transportVideo(t, "mp4")
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("media credentials reached another host")
		}
		http.ServeFile(w, r, path)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, cookieErr := r.Cookie("media")
		if cookieErr != nil || cookie.Value != "allowed" {
			http.Error(w, "missing cookie", 403)
			return
		}
		http.Redirect(w, r, strings.Replace(target.URL, "127.0.0.1", "localhost", 1)+"/video.mp4", http.StatusFound)
	}))
	defer origin.Close()
	headers := http.Header{"Cookie": {"media=allowed"}, "User-Agent": {"browser-agent"}}
	if _, err := ProbeInput(WithHTTPProxy(context.Background(), ""), origin.URL+"/video.mp4", MediaRequestArgs("ffprobe", origin.URL+"/video.mp4", "mp4", headers)); err != nil {
		t.Fatal(err)
	}
}
