package mediacheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func transportVideo(t *testing.T, format string) string {
	t.Helper()
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}
	p := filepath.Join(t.TempDir(), "video."+format)
	args := []string{"-v", "error", "-f", "lavfi", "-i", "color=c=black:s=160x90:r=2", "-t", "3", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1"}
	if format == "payload" {
		args = append(args, "-f", "mpegts")
	} else {
		args = append(args, "-movflags", "+faststart")
	}
	out, err := exec.Command("ffmpeg", append(args, p)...).CombinedOutput()
	if err != nil {
		t.Fatalf("video fixture: %v %s", err, out)
	}
	return p
}
func TestHLSPreflightSupportsDownloaderInputs(t *testing.T) {
	p := transportVideo(t, "payload")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() != "https://player.invalid/" || r.UserAgent() != "fixture-agent" {
			http.Error(w, "headers required", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/index.m3u8" {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:3.0,\nsegment.payload\n#EXT-X-ENDLIST\n")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, p)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = WithHTTPProxy(ctx, "")
	headers := InputArgs("https://player.invalid/", "fixture-agent")
	supported := append([]string{"-extension_picky", "0", "-allowed_extensions", "ALL", "-allowed_segment_extensions", "ALL"}, headers...)
	if _, err := ProbeInput(ctx, srv.URL+"/index.m3u8", supported); err != nil {
		t.Fatalf("supported input control failed: %v", err)
	}
	if _, err := ProbeInput(ctx, srv.URL+"/index.m3u8", headers); err != nil {
		t.Fatalf("preflight rejects a valid HLS accepted with downloader flags: %v", err)
	}
	// OCR frames and full stream-copy transfers use the same headers and HLS
	// options, selected for each installed tool rather than a version assumption.
	for _, output := range [][]string{{"-frames:v", "1", "-f", "null", "-"}, {"-c", "copy", "-f", "null", "-"}} {
		args := append([]string{"-v", "error"}, MediaInputArgs("ffmpeg", srv.URL+"/index.m3u8", "m3u8", "https://player.invalid/", "fixture-agent")...)
		args = append(args, "-i", srv.URL+"/index.m3u8")
		args = append(args, output...)
		if out, err := MediaCommand(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
			t.Fatalf("HLS frame/transfer: %v %s", err, out)
		}
	}
}

func TestProbeDiagnosticRedactsSignedURLs(t *testing.T) {
	err := &exec.ExitError{Stderr: []byte("HTTP 403: https://user:secret@cdn.invalid/video?token=private\ninvalid segment")}
	detail := probeDiagnostic(err)
	if !strings.Contains(detail, "HTTP 403") || strings.Contains(detail, "secret") || strings.Contains(detail, "private") {
		t.Fatal(detail)
	}
}
func TestSOCKSRouteNeverFallsThroughToDirect(t *testing.T) {
	p := transportVideo(t, "mp4")
	var direct atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { direct.Add(1); http.ServeFile(w, r, p) }))
	defer origin.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var proxied atomic.Int32
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			proxied.Add(1)
			c.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, controlErr := ProbeInput(WithHTTPProxy(ctx, "http://"+ln.Addr().String()), origin.URL+"/video.mp4", nil)
	if controlErr == nil || proxied.Load() == 0 {
		t.Fatal("HTTP proxy control did not reach configured listener")
	}
	proxied.Store(0)
	direct.Store(0)
	_, err = ProbeInput(WithHTTPProxy(ctx, "socks5://"+ln.Addr().String()), origin.URL+"/video.mp4", nil)
	if err == nil || direct.Load() != 0 {
		t.Fatal("unprepared SOCKS5 route must fail without reaching the origin")
	}
}
