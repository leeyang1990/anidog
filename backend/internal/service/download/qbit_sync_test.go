package download

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

func TestQBitSyncLoginAcceptsNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/auth/login" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	s := &QBitSyncer{baseURL: server.URL, user: "admin", pass: "secret", client: server.Client()}
	if err := s.ensureLogin(context.Background()); err != nil {
		t.Fatalf("204 login should succeed: %v", err)
	}
}

func TestShouldAbandonStalledDownload(t *testing.T) {
	now := time.Now()
	stale := now.Add(-stalledProgressTimeout - time.Minute)
	recent := now.Add(-time.Minute)
	base := map[string]interface{}{
		"has_metadata": true,
		"progress":     0.1519,
		"dlspeed":      float64(0),
		"num_seeds":    float64(0),
		"availability": 0.1519,
	}
	if !shouldAbandonStalledDownload("stalledDL", base, &stale, now) {
		t.Fatal("a stale incomplete swarm should be abandoned")
	}
	if shouldAbandonStalledDownload("stalledDL", base, &recent, now) {
		t.Fatal("a recently progressing torrent must be kept")
	}
	withAvailablePieces := cloneTorrentMap(base)
	withAvailablePieces["availability"] = 0.8
	if shouldAbandonStalledDownload("stalledDL", withAvailablePieces, &stale, now) {
		t.Fatal("a swarm with downloadable pieces must be kept")
	}
	withSeed := cloneTorrentMap(base)
	withSeed["num_seeds"] = float64(1)
	if shouldAbandonStalledDownload("stalledDL", withSeed, &stale, now) {
		t.Fatal("a torrent with a connected seed must be kept")
	}
}

func TestDownloadedBytesAreThePrimaryProgressSignal(t *testing.T) {
	previousBytes := int64(10_000)
	dl := &model.Download{DownloadedBytes: &previousBytes, Progress: 7.72}
	torrent := map[string]interface{}{
		"downloaded": float64(11_000),
		"progress":   0.0755,
	}
	if !downloadProgressAdvanced(dl, torrent) {
		t.Fatal("increasing downloaded bytes must count as progress even when percentage decreases")
	}
}

func TestEnsureQueuePolicy(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/app/setPreferences" {
			http.NotFound(w, r)
			return
		}
		calls++
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		var prefs map[string]interface{}
		if err := json.Unmarshal([]byte(values.Get("json")), &prefs); err != nil {
			t.Fatal(err)
		}
		if enabled, _ := prefs["dont_count_slow_torrents"].(bool); !enabled {
			t.Fatalf("unexpected preferences: %s", strings.TrimSpace(values.Get("json")))
		}
		if got := int(prefs["max_active_downloads"].(float64)); got != defaultMaxActiveDownloads {
			t.Fatalf("max_active_downloads=%d, want %d", got, defaultMaxActiveDownloads)
		}
		if got := int(prefs["max_active_torrents"].(float64)); got != defaultMaxActiveTorrents {
			t.Fatalf("max_active_torrents=%d, want %d", got, defaultMaxActiveTorrents)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := &QBitSyncer{baseURL: server.URL, client: server.Client()}
	s.ensureQueuePolicy(context.Background())
	s.ensureQueuePolicy(context.Background())
	if calls != 1 {
		t.Fatalf("queue policy should be applied once, got %d calls", calls)
	}
}

func cloneTorrentMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
