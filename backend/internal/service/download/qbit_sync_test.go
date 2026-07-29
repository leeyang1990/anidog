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

func TestObserveLongTermHealthFlagsSlowTorrentWithoutFastPeerEvidence(t *testing.T) {
	now := time.Now()
	startBytes := int64(20_000_000)
	torrent := unavailableTorrent(0.25, 21_000_000)
	torrent["num_seeds"] = float64(1)
	torrent["availability"] = 1.0
	torrent["dlspeed"] = float64(512)

	twoHoursAgo := now.Add(-longTermSpeedWindow - time.Minute)
	dl := &model.Download{
		SpeedWindowStartedAt:  &twoHoursAgo,
		SpeedWindowStartBytes: &startBytes,
	}
	if _, reason := observeLongTermTorrentHealth(dl, torrent, now); !strings.Contains(reason, "不可用慢种") {
		t.Fatalf("expected non-destructive slow-torrent signal, got %q", reason)
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

func TestInitialQualityProbeRejectsIncompleteSwarm(t *testing.T) {
	now := time.Now()
	started := now.Add(-initialQualityProbeWindow - time.Minute)
	startBytes := int64(10_000_000)
	dl := &model.Download{
		QualityProbeStartedAt:  &started,
		QualityProbeStartBytes: &startBytes,
	}
	torrent := unavailableTorrent(0.06, 10_100_000)
	torrent["state"] = "stalledDL"

	_, reason := observeInitialTorrentQuality(dl, torrent, now)
	if !strings.Contains(reason, "无法拼出完整文件") {
		t.Fatalf("expected incomplete swarm rejection, got %q", reason)
	}
}

func TestInitialQualityProbeFlagsSlowTorrentWithoutDestructiveCorroboration(t *testing.T) {
	now := time.Now()
	started := now.Add(-initialQualityProbeWindow - time.Minute)
	startBytes := int64(10_000_000)
	dl := &model.Download{
		QualityProbeStartedAt:  &started,
		QualityProbeStartBytes: &startBytes,
	}
	torrent := unavailableTorrent(0.25, 10_100_000)
	torrent["state"] = "downloading"
	torrent["num_seeds"] = float64(2)
	torrent["availability"] = 2.5
	torrent["dlspeed"] = float64(1024)

	if _, reason := observeInitialTorrentQuality(dl, torrent, now); !strings.Contains(reason, "真实吞吐") {
		t.Fatalf("expected slow peer race signal, got %q", reason)
	}
}

func TestInitialQualityProbeDoesNotCountQueuedTime(t *testing.T) {
	now := time.Now()
	started := now.Add(-time.Hour)
	startBytes := int64(0)
	dl := &model.Download{
		QualityProbeStartedAt:  &started,
		QualityProbeStartBytes: &startBytes,
	}
	torrent := unavailableTorrent(0, 0)
	torrent["state"] = "queuedDL"

	updates, reason := observeInitialTorrentQuality(dl, torrent, now)
	if reason != "" || len(updates) != 0 {
		t.Fatalf("queued time must not fail the quality probe: updates=%#v reason=%q", updates, reason)
	}
}

func TestWastedQualityReasonRequiresSlowCorroboratedTorrent(t *testing.T) {
	now := time.Now()
	started := now.Add(-time.Hour)
	downloaded := int64(300 * 1024 * 1024)
	wasted := int64(90 * 1024 * 1024)
	dl := &model.Download{
		DownloadedBytes:       &downloaded,
		TotalWastedBytes:      &wasted,
		QualityProbeStartedAt: &started,
	}
	torrent := unavailableTorrent(0.30, downloaded)
	torrent["dlspeed"] = float64(1024)

	reason := wastedQualityReason(dl, torrent, now, true)
	if !strings.Contains(reason, "浪费流量") {
		t.Fatalf("expected wasted traffic rejection, got %q", reason)
	}
	torrent["dlspeed"] = float64(128 * 1024)
	if reason := wastedQualityReason(dl, torrent, now, true); reason != "" {
		t.Fatalf("fast torrent must not be rejected for historical waste: %q", reason)
	}
}

func TestGetTotalWasted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrents/properties" || r.URL.Query().Get("hash") != "abcdef" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_wasted": 94_371_840})
	}))
	defer server.Close()

	s := &QBitSyncer{baseURL: server.URL, client: server.Client()}
	got, err := s.getTotalWasted(context.Background(), "ABCDEF")
	if err != nil {
		t.Fatal(err)
	}
	if got != 94_371_840 {
		t.Fatalf("total_wasted=%d", got)
	}
}

func TestQualityReplacementCooldown(t *testing.T) {
	db := testutil.InitTestDB()
	now := time.Now()
	s := &QBitSyncer{db: db}
	if !s.alternativeSearchAllowed(context.Background(), now) {
		t.Fatal("alternative search should initially be allowed")
	}
	searchAt := now.Add(-time.Minute)
	row := model.Download{
		TorrentID:           "slow-candidate",
		Name:                "slow",
		URL:                 "magnet:?xt=urn:btih:ABCDEF",
		Status:              model.DownloadStatusDownloading,
		DownloadType:        model.DownloadTypeTorrent,
		AlternativeSearchAt: &searchAt,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if s.alternativeSearchAllowed(context.Background(), now) {
		t.Fatal("alternative search must be rate-limited during cooldown")
	}
}

func TestKeepSlowTorrentStartsAlternativeSearchWithoutDeletingIt(t *testing.T) {
	db := testutil.InitTestDB()
	animeID, episode := uint(7), 2
	hash := "ABCDEF0123456789ABCDEF0123456789ABCDEF01"
	row := model.Download{
		TorrentID:     "slow-kept",
		Name:          "episode 2",
		URL:           "magnet:?xt=urn:btih:" + hash,
		Status:        model.DownloadStatusDownloading,
		DownloadType:  model.DownloadTypeTorrent,
		AnimeID:       &animeID,
		EpisodeNumber: &episode,
		InfoHash:      &hash,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	called := make(chan struct{}, 1)
	s := &QBitSyncer{db: db}
	s.SetSlowTorrentHandler(func(_ context.Context, gotAnimeID uint, gotEpisode int) {
		if gotAnimeID != animeID || gotEpisode != episode {
			t.Errorf("unexpected search target anime=%d episode=%d", gotAnimeID, gotEpisode)
		}
		called <- struct{}{}
	})
	if !s.keepSlowTorrentAndSearch(context.Background(), &row, "平均速度过低") {
		t.Fatal("slow torrent should start an alternative search")
	}
	var saved model.Download
	if err := db.First(&saved, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.DownloadStatusDownloading || !saved.SeekingAlternative ||
		saved.AlternativeSearchAt == nil {
		t.Fatalf("slow torrent was not retained correctly: %#v", saved)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("alternative search callback was not triggered")
	}
	if s.keepSlowTorrentAndSearch(context.Background(), &saved, "still slow") {
		t.Fatal("an already-racing slow torrent must not retrigger the callback")
	}
}

func TestCompletedCandidateCleansUpOnlyItsIncompleteSiblings(t *testing.T) {
	db := testutil.InitTestDB()
	animeID, episode := uint(7), 2
	winnerHash := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	slowHash := "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	winner := model.Download{
		TorrentID: "winner", Name: "winner", URL: "magnet:?xt=urn:btih:" + winnerHash,
		Status: model.DownloadStatusCompleted, DownloadType: model.DownloadTypeTorrent,
		AnimeID: &animeID, EpisodeNumber: &episode, InfoHash: &winnerHash,
	}
	slow := model.Download{
		TorrentID: "slow", Name: "slow", URL: "magnet:?xt=urn:btih:" + slowHash,
		Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeTorrent,
		AnimeID: &animeID, EpisodeNumber: &episode, InfoHash: &slowHash,
		SeekingAlternative: true,
	}
	if err := db.Create(&winner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&slow).Error; err != nil {
		t.Fatal(err)
	}
	deletedHash := ""
	deleteFiles := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrents/delete" {
			http.NotFound(w, r)
			return
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		values, _ := url.ParseQuery(string(body))
		deletedHash = values.Get("hashes")
		deleteFiles = values.Get("deleteFiles")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := &QBitSyncer{db: db, baseURL: server.URL, client: server.Client()}
	s.cancelSiblingTorrents(context.Background(), &winner)
	if deletedHash != strings.ToLower(slowHash) {
		t.Fatalf("deleted hash=%q", deletedHash)
	}
	if deleteFiles != "false" {
		t.Fatalf("race cleanup must preserve files, deleteFiles=%q", deleteFiles)
	}
	var saved model.Download
	if err := db.First(&saved, slow.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.DownloadStatusSuperseded || saved.SeekingAlternative {
		t.Fatalf("sibling race state not closed: %#v", saved)
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

func TestSyncRejectsMissingQBitTaskAndImmediatelyRecoversEpisode(t *testing.T) {
	db := testutil.InitTestDB()
	hash := "9AC43785313D3F97B6B6F165B708A7DDB95F092B"
	animeID, episode := uint(1), 3
	nextRetry := time.Now().Add(time.Hour)
	wrongBatchSize := int64(45_210_546_716)
	partialBytes := int64(1_024)
	row := model.Download{
		TorrentID:       "legacy-wrong-season",
		Name:            "无职转生 第3季 - 第03集",
		URL:             "magnet:?xt=urn:btih:" + hash,
		Status:          model.DownloadStatusDownloading,
		DownloadType:    model.DownloadTypeTorrent,
		InfoHash:        &hash,
		AnimeID:         &animeID,
		EpisodeNumber:   &episode,
		FailureKind:     model.FailureKindTransient,
		LastError:       "季度不匹配：错误选中第二季合集，已自动删除",
		NextRetryAt:     &nextRetry,
		Progress:        12.5,
		TotalBytes:      &wrongBatchSize,
		DownloadedBytes: &partialBytes,
		CreatedAt:       time.Now().Add(-2 * time.Hour),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/app/setPreferences":
			w.WriteHeader(http.StatusOK)
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	recovered := make(chan struct{}, 1)
	s := &QBitSyncer{db: db, baseURL: server.URL, user: "admin", pass: "secret", client: server.Client()}
	s.SetDeadTorrentHandler(func(_ context.Context, gotAnimeID uint, gotEpisode int) {
		if gotAnimeID != animeID || gotEpisode != episode {
			t.Errorf("unexpected recovery target: anime=%d episode=%d", gotAnimeID, gotEpisode)
		}
		recovered <- struct{}{}
	})

	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}

	var saved model.Download
	if err := db.First(&saved, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.DownloadStatusFailed || saved.FailureKind != model.FailureKindRejected {
		t.Fatalf("missing qBit task must be rejected, got status=%s kind=%s", saved.Status, saved.FailureKind)
	}
	if saved.NextRetryAt != nil {
		t.Fatalf("rejected candidate must not retain retry cooldown: %v", saved.NextRetryAt)
	}
	if saved.TotalBytes != nil || saved.DownloadedBytes != nil || saved.Progress != 0 {
		t.Fatalf("rejected candidate retained stale progress: total=%v downloaded=%v progress=%v",
			saved.TotalBytes, saved.DownloadedBytes, saved.Progress)
	}
	if !strings.Contains(saved.LastError, "季度不匹配") ||
		!strings.Contains(saved.LastError, "qBittorrent 中不存在") {
		t.Fatalf("original cause must be preserved, got %q", saved.LastError)
	}
	select {
	case <-recovered:
	case <-time.After(time.Second):
		t.Fatal("episode recovery was not triggered")
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
		if got := int64(prefs["dl_limit"].(float64)); got != 2048*1024 {
			t.Fatalf("dl_limit=%d, want %d", got, 2048*1024)
		}
		if got := int64(prefs["up_limit"].(float64)); got != 512*1024 {
			t.Fatalf("up_limit=%d, want %d", got, 512*1024)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := &QBitSyncer{baseURL: server.URL, client: server.Client()}
	s.SetRateLimitLoader(func(context.Context) (int64, int64) {
		return 2048, 512
	})
	s.ensureQueuePolicy(context.Background())
	s.ensureQueuePolicy(context.Background())
	if calls != 1 {
		t.Fatalf("queue policy should be applied once, got %d calls", calls)
	}
	s.InvalidatePreferences()
	s.ensureQueuePolicy(context.Background())
	if calls != 2 {
		t.Fatalf("invalidated policy should be applied again, got %d calls", calls)
	}
}

func TestQBitRateLimitBytes(t *testing.T) {
	if got := qbitRateLimitBytes(0); got != 0 {
		t.Fatalf("unlimited got %d", got)
	}
	if got := qbitRateLimitBytes(-1); got != 0 {
		t.Fatalf("negative limit got %d", got)
	}
	if got := qbitRateLimitBytes(1024); got != 1024*1024 {
		t.Fatalf("1 MiB/s got %d", got)
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
