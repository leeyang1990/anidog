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
	"github.com/anidog/anidog-go/internal/testutil"
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

func TestObserveLongTermHealthTinyTrickleDoesNotResetStall(t *testing.T) {
	now := time.Now()
	stalledSince := now.Add(-continuousUnavailableTimeout - time.Minute)
	lastProgress := now.Add(-time.Minute) // a tiny recent trickle
	dl := &model.Download{StalledSince: &stalledSince, LastProgressAt: &lastProgress}
	torrent := unavailableTorrent(0.25, 25_000_000)

	_, reason := observeLongTermTorrentHealth(dl, torrent, now)
	if !strings.Contains(reason, "偶发字节不重置") {
		t.Fatalf("expected continuous-stall rejection, got %q", reason)
	}
}

func TestObserveLongTermHealthRejectsLowAverageSpeed(t *testing.T) {
	now := time.Now()
	windowStarted := now.Add(-longTermSpeedWindow - time.Minute)
	stalledSince := now.Add(-time.Hour)
	startBytes := int64(20_000_000)
	dl := &model.Download{
		StalledSince:          &stalledSince,
		SpeedWindowStartedAt:  &windowStarted,
		SpeedWindowStartBytes: &startBytes,
	}
	torrent := unavailableTorrent(0.25, 21_000_000)

	_, reason := observeLongTermTorrentHealth(dl, torrent, now)
	if !strings.Contains(reason, "长期平均速度") {
		t.Fatalf("expected long-term speed rejection, got %q", reason)
	}
}

func TestObserveLongTermHealthRejectsConnectedButUnusablySlowSeed(t *testing.T) {
	now := time.Now()
	windowStarted := now.Add(-longTermSpeedWindow - time.Minute)
	startBytes := int64(20_000_000)
	dl := &model.Download{
		SpeedWindowStartedAt:  &windowStarted,
		SpeedWindowStartBytes: &startBytes,
	}
	torrent := unavailableTorrent(0.25, 21_000_000)
	torrent["num_seeds"] = float64(1)
	torrent["availability"] = 1.0
	torrent["dlspeed"] = float64(512)

	_, reason := observeLongTermTorrentHealth(dl, torrent, now)
	if !strings.Contains(reason, "不可用慢种") {
		t.Fatalf("expected connected but unusably slow seed rejection, got %q", reason)
	}
}

func TestObserveLongTermHealthResetsWhenCompleteSeedConnects(t *testing.T) {
	now := time.Now()
	stalledSince := now.Add(-continuousUnavailableTimeout - time.Minute)
	dl := &model.Download{StalledSince: &stalledSince}
	torrent := unavailableTorrent(0.25, 25_000_000)
	torrent["num_seeds"] = float64(1)
	torrent["availability"] = 1.0

	updates, reason := observeLongTermTorrentHealth(dl, torrent, now)
	if reason != "" {
		t.Fatalf("healthy swarm should be kept: %s", reason)
	}
	if value, exists := updates["stalled_since"]; !exists || value != nil {
		t.Fatalf("stalled_since should be cleared, got %#v", updates)
	}
}

func TestSetForceStart(t *testing.T) {
	var gotValues url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrents/setForceStart" {
			http.NotFound(w, r)
			return
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		gotValues, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := &QBitSyncer{baseURL: server.URL, client: server.Client()}
	if err := s.setForceStart(context.Background(), "ABCDEF", true); err != nil {
		t.Fatal(err)
	}
	if gotValues.Get("hashes") != "abcdef" || gotValues.Get("value") != "true" {
		t.Fatalf("unexpected request values: %#v", gotValues)
	}
}

func TestSyncStartsMetadataProbeForQueuedMagnet(t *testing.T) {
	db := testutil.InitTestDB()
	hash := "ABCDEF0123456789"
	row := model.Download{
		TorrentID:    "probe-task",
		Name:         "queued magnet",
		URL:          "magnet:?xt=urn:btih:" + hash,
		Status:       model.DownloadStatusPending,
		DownloadType: model.DownloadTypeTorrent,
		InfoHash:     &hash,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	forceCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/app/setPreferences":
			w.WriteHeader(http.StatusOK)
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{{
				"hash": hash, "state": "queuedDL", "has_metadata": false,
				"progress": float64(0), "downloaded": float64(0),
				"dlspeed": float64(0), "num_seeds": float64(0),
				"availability": float64(0),
			}})
		case "/api/v2/torrents/setForceStart":
			forceCalls++
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := &QBitSyncer{db: db, baseURL: server.URL, user: "admin", pass: "secret", client: server.Client()}
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	var saved model.Download
	if err := db.First(&saved, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if forceCalls != 1 || saved.MetadataProbeStartedAt == nil {
		t.Fatalf("metadata probe not started: calls=%d started_at=%v", forceCalls, saved.MetadataProbeStartedAt)
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

func unavailableTorrent(progress float64, downloaded int64) map[string]interface{} {
	return map[string]interface{}{
		"has_metadata": true,
		"progress":     progress,
		"downloaded":   float64(downloaded),
		"dlspeed":      float64(0),
		"num_seeds":    float64(0),
		"availability": progress,
	}
}

func cloneTorrentMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
