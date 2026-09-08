package embedded

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
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

	restored := newTestEngine(t, leecherState, downloadDir)
	defer restored.Close()
	items, err := restored.ListTorrents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != leecherHash || items[0].State != "pausedDL" {
		t.Fatalf("manifest restore mismatch: %#v", items)
	}
	if err := restored.RemoveTorrent(ctx, leecherHash, true); err != nil {
		t.Fatal(err)
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
