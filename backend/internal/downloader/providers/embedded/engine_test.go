package embedded

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"

	"github.com/anidog/anidog-go/internal/config"
)

func TestEngineDownloadsFromLocalPeerAndRestoresManifest(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	root := t.TempDir()
	seedDir := filepath.Join(root, "seed")
	downloadDir := filepath.Join(root, "download")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := bytes.Repeat([]byte("AniDog embedded torrent engine\n"), 64*1024)
	fileName := "engine-fixture.bin"
	if err := os.WriteFile(filepath.Join(seedDir, fileName), want, 0o644); err != nil {
		t.Fatal(err)
	}
	torrentFile := buildTorrentFile(t, filepath.Join(seedDir, fileName), root)

	seed := newTestEngine(t, filepath.Join(root, "seed-state"), seedDir)
	defer seed.Close()
	seedHash, err := seed.AddTorrent(ctx, torrentFile, seedDir)
	if err != nil {
		t.Fatalf("seed add: %v", err)
	}

	leecherState := filepath.Join(root, "leecher-state")
	leecher := newTestEngine(t, leecherState, downloadDir)
	leecherHash, err := leecher.AddTorrent(ctx, torrentFile, downloadDir)
	if err != nil {
		t.Fatalf("leecher add: %v", err)
	}
	if leecherHash != seedHash {
		t.Fatalf("hash mismatch: seed=%s leecher=%s", seedHash, leecherHash)
	}
	duplicateDir := filepath.Join(root, "duplicate-must-not-relocate")
	if _, err := leecher.AddTorrent(ctx, torrentFile, duplicateDir); err == nil {
		t.Fatal("duplicate with different save path must be rejected")
	}
	duplicateHash, err := leecher.AddTorrent(ctx, torrentFile, downloadDir)
	if err != nil || duplicateHash != leecherHash {
		t.Fatalf("same-path duplicate: %s %v", duplicateHash, err)
	}
	leecher.mu.RLock()
	trackedSavePath := leecher.entries[leecherHash].SavePath
	leecher.mu.RUnlock()
	if trackedSavePath != downloadDir {
		t.Fatalf("duplicate add relocated existing task: %s", trackedSavePath)
	}

	leecher.mu.RLock()
	added := leecher.entries[leecherHash].torrent.AddClientPeer(seed.client)
	leecher.mu.RUnlock()
	if added == 0 {
		t.Fatal("local peer was not added")
	}

	waitFor(t, ctx, func() bool {
		items, err := leecher.ListTorrents(ctx)
		return err == nil && len(items) == 1 && items[0].Progress == 1 && items[0].State == "uploading"
	})
	got, err := os.ReadFile(filepath.Join(downloadDir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("downloaded bytes differ from source")
	}

	if err := leecher.PauseTorrent(ctx, leecherHash); err != nil {
		t.Fatal(err)
	}
	if err := leecher.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(torrentFile); err != nil {
		t.Fatal(err)
	}

	restored := newTestEngine(t, leecherState, downloadDir)
	defer restored.Close()
	items, err := restored.ListTorrents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != leecherHash || items[0].State != "pausedDL" ||
		!items[0].HasMetadata || items[0].Size != int64(len(want)) {
		t.Fatalf("manifest restore mismatch: %#v", items)
	}
	cachedMetainfo := restored.metainfoPath(leecherHash)
	if _, err := os.Stat(cachedMetainfo); err != nil {
		t.Fatalf("cached metainfo missing: %v", err)
	}
	if err := restored.RemoveTorrent(ctx, leecherHash, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cachedMetainfo); !os.IsNotExist(err) {
		t.Fatalf("cached metainfo was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(downloadDir, fileName)); !os.IsNotExist(err) {
		t.Fatalf("torrent data was not removed: %v", err)
	}
}

func TestEngineHTTPProxyCanBeUpdatedAtRuntime(t *testing.T) {
	engine := newTestEngine(t, filepath.Join(t.TempDir(), "state"), t.TempDir())
	defer engine.Close()

	if err := engine.SetHTTPProxy(context.Background(), "http://127.0.0.1:7890"); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, "https://example.test", nil)
	got, err := engine.proxy.proxy(request)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.String() != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy: %v", got)
	}
	if err := engine.SetHTTPProxy(context.Background(), "://bad"); err == nil {
		t.Fatal("invalid proxy was accepted")
	}
}

func TestEngineHealthTracksLifecycle(t *testing.T) {
	engine := newTestEngine(t, filepath.Join(t.TempDir(), "state"), t.TempDir())
	health := engine.Health(context.Background())
	if !health.Online || health.Name != "AniDog Embedded BT" || health.ListenPort <= 0 {
		t.Fatalf("unexpected live engine health: %#v", health)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	health = engine.Health(context.Background())
	if health.Online {
		t.Fatalf("closed engine still reported online: %#v", health)
	}
}

func TestEngineCachesMagnetMetadataAfterDiscovery(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	root := t.TempDir()
	seedDir := filepath.Join(root, "seed")
	downloadDir := filepath.Join(root, "download")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := bytes.Repeat([]byte("AniDog magnet metadata cache\n"), 16*1024)
	sourcePath := filepath.Join(seedDir, "magnet-fixture.bin")
	if err := os.WriteFile(sourcePath, want, 0o644); err != nil {
		t.Fatal(err)
	}
	torrentFile := buildTorrentFile(t, sourcePath, root)
	mi, err := metainfo.LoadFromFile(torrentFile)
	if err != nil {
		t.Fatal(err)
	}

	seed := newTestEngine(t, filepath.Join(root, "seed-state"), seedDir)
	defer seed.Close()
	seedHash, err := seed.AddTorrent(ctx, torrentFile, seedDir)
	if err != nil {
		t.Fatal(err)
	}

	leecherState := filepath.Join(root, "leecher-state")
	leecher := newTestEngine(t, leecherState, downloadDir)
	magnetURL := mi.Magnet(nil, nil).String()
	leecherHash, err := leecher.AddTorrent(ctx, magnetURL, downloadDir)
	if err != nil {
		t.Fatal(err)
	}
	if leecherHash != seedHash {
		t.Fatalf("hash mismatch: seed=%s leecher=%s", seedHash, leecherHash)
	}
	leecher.mu.RLock()
	added := leecher.entries[leecherHash].torrent.AddClientPeer(seed.client)
	leecher.mu.RUnlock()
	if added == 0 {
		t.Fatal("local peer was not added")
	}

	waitFor(t, ctx, func() bool {
		_, err := os.Stat(leecher.metainfoPath(leecherHash))
		return err == nil
	})
	if err := leecher.PauseTorrent(ctx, leecherHash); err != nil {
		t.Fatal(err)
	}
	if err := leecher.Close(); err != nil {
		t.Fatal(err)
	}

	restored := newTestEngine(t, leecherState, downloadDir)
	defer restored.Close()
	items, err := restored.ListTorrents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].HasMetadata || items[0].ID != leecherHash ||
		items[0].State != "pausedDL" || items[0].Size != int64(len(want)) {
		t.Fatalf("cached magnet restore mismatch: %#v", items)
	}
}

func newTestEngine(t *testing.T, stateDir, mediaRoot string) *Engine {
	t.Helper()
	engine, err := New(&config.Config{
		BTListenPort: 0,
		BTStateDir:   stateDir,
		BTSeed:       true,
		MediaRoot:    mediaRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func buildTorrentFile(t *testing.T, sourcePath, outputDir string) string {
	t.Helper()
	info := metainfo.Info{PieceLength: 32 * 1024}
	if err := info.BuildFromFilePath(sourcePath); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	mi := metainfo.MetaInfo{InfoBytes: infoBytes, CreatedBy: "AniDog test"}
	mi.SetDefaults()
	path := filepath.Join(outputDir, "fixture.torrent")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := mi.Write(file); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitFor(t *testing.T, ctx context.Context, ready func() bool) {
	t.Helper()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ready() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestDuplicateTorrentRegistrationSurvivesMetadataOutageAndRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "fixture.bin")
	if err := os.WriteFile(source, []byte("managed payload"), 0600); err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(buildTorrentFile(t, source, root))
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	var offline atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if offline.Load() {
			http.Error(w, "metadata origin unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(metadata)
	}))
	defer server.Close()
	state, media := filepath.Join(root, "state"), filepath.Join(root, "media")
	engine := newTestEngine(t, state, media)
	defer func() { _ = engine.Close() }()
	url := server.URL + "/release.torrent"
	hash, err := engine.AddTorrent(ctx, url, media)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.PauseTorrent(ctx, hash); err != nil {
		t.Fatal(err)
	}
	offline.Store(true)
	check := func(e *Engine) {
		t.Helper()
		got, err := e.AddTorrent(ctx, url, media)
		if err != nil || got != hash {
			t.Fatalf("managed registration depended on metadata origin: %s %v", got, err)
		}
		if _, err := e.AddTorrent(ctx, url, filepath.Join(root, "other")); err == nil {
			t.Fatal("duplicate registration relocated managed media")
		}
		items, err := e.ListTorrents(ctx)
		if err != nil || len(items) != 1 || items[0].ID != hash || items[0].State != "pausedDL" {
			t.Fatalf("registration changed managed task or pause state: %+v %v", items, err)
		}
		if requests.Load() != 1 {
			t.Fatalf("duplicate registration fetched metadata again: %d", requests.Load())
		}
	}
	check(engine)
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine = newTestEngine(t, state, media)
	check(engine)
}

func TestEngineRechecksAndRepairsRetainedPartialAfterRestart(t *testing.T) {
	if os.Getenv("ANIDOG_PARTIAL_RESTART_TEST") != "1" {
		for _, mode := range []string{"classic", "mmap"} {
			t.Run(mode, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEngineRechecksAndRepairsRetainedPartialAfterRestart$")
				for _, value := range os.Environ() {
					if !strings.HasPrefix(value, "TORRENT_STORAGE_DEFAULT_FILE_IO=") && !strings.HasPrefix(value, "ANIDOG_PARTIAL_RESTART_TEST=") {
						cmd.Env = append(cmd.Env, value)
					}
				}
				cmd.Env = append(cmd.Env, "TORRENT_STORAGE_DEFAULT_FILE_IO="+mode, "ANIDOG_PARTIAL_RESTART_TEST=1")
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("retained partial with %s IO: %v\n%s", mode, err, output)
				}
			})
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := t.TempDir()
	seedDir, downloadDir := filepath.Join(root, "seed"), filepath.Join(root, "download")
	for _, dir := range []string{seedDir, downloadDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	want := make([]byte, 2<<20)
	for i := range want {
		want[i] = byte((i/(32<<10) + i%251) % 251)
	}
	const name = "retained.bin"
	if err := os.WriteFile(filepath.Join(seedDir, name), want, 0600); err != nil {
		t.Fatal(err)
	}
	metadata := buildTorrentFile(t, filepath.Join(seedDir, name), root)
	seed := newTestEngine(t, filepath.Join(root, "seed-state"), seedDir)
	defer seed.Close()
	if _, err := seed.AddTorrent(ctx, metadata, seedDir); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(downloadDir, name+".part")
	if err := os.WriteFile(partial, want[:256<<10], 0600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "download-state")
	first := newTestEngine(t, state, downloadDir)
	hash, err := first.AddTorrent(ctx, metadata, downloadDir)
	if err != nil {
		t.Fatal(err)
	}
	// Finish the initial scan before taking the retained-data snapshot. The
	// scenario under test is rechecking a previously stopped partial download.
	waitFor(t, ctx, func() bool {
		first.mu.RLock()
		defer first.mu.RUnlock()
		for _, state := range first.entries[hash].torrent.PieceStateRuns() {
			if state.Hashing || state.QueuedForHash || state.Marking {
				return false
			}
		}
		return true
	})
	if err := first.PauseTorrent(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(partial, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{want[0] ^ 0xff}, 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(metadata); err != nil {
		t.Fatal(err)
	}
	restored := newTestEngine(t, state, downloadDir)
	defer restored.Close()
	if err := restored.ResumeTorrent(ctx, hash); err != nil {
		t.Fatal(err)
	}
	waitFor(t, ctx, func() bool {
		rows, err := restored.ListTorrents(ctx)
		return err == nil && len(rows) == 1 && rows[0].Downloaded >= (256<<10)-(32<<10) && rows[0].Progress < 1
	})
	restored.mu.RLock()
	added := restored.entries[hash].torrent.AddClientPeer(seed.client)
	restored.mu.RUnlock()
	if added == 0 {
		t.Fatal("local seed was not connected")
	}
	waitFor(t, ctx, func() bool {
		rows, err := restored.ListTorrents(ctx)
		return err == nil && len(rows) == 1 && rows[0].Progress == 1
	})
	got, err := os.ReadFile(filepath.Join(downloadDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("retained corruption survived piece verification")
	}
}
