package embedded

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
)

func TestNASCleanupDoesNotHoldEngineControlLock(t *testing.T) {
	root := t.TempDir()
	engine := newTestEngine(t, filepath.Join(root, "state"), root)
	defer engine.Close()
	hash, err := engine.AddTorrent(context.Background(), "magnet:?xt=urn:btih:"+strings.Repeat("9", 40), filepath.Join(root, ".anidog-race", "task"))
	if err != nil {
		t.Fatal(err)
	}
	entered, release, removed := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	engine.removeData = func(string, string) error { close(entered); <-release; return errors.New("NAS unavailable") }
	go func() { removed <- engine.RemoveTorrent(context.Background(), hash, true) }()
	<-entered
	health := make(chan bool, 1)
	go func() { health <- engine.Health(context.Background()).Online }()
	select {
	case online := <-health:
		if !online {
			t.Error("engine was taken offline by cleanup")
		}
	case <-time.After(time.Second):
		t.Error("NAS cleanup blocked engine health/control")
	}
	close(release)
	if err := <-removed; err != nil {
		t.Fatal(err)
	}
}

func TestRemovalCleanupFailureIsJournaledWithoutResurrectingTorrent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	state := filepath.Join(root, "state")
	engine := newTestEngine(t, state, root)
	first := strings.Repeat("A", 40)
	second := strings.Repeat("B", 40)
	staging := filepath.Join(root, ".anidog-race", first)
	for _, hash := range []string{first, second} {
		if _, err := engine.AddTorrent(ctx, "magnet:?xt=urn:btih:"+hash, filepath.Join(root, ".anidog-race", hash)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(staging, ".torrent.bolt.db"), []byte("NAS residue"), 0600); err != nil {
		t.Fatal(err)
	}
	engine.removeData = func(string, string) error { return errors.New("directory not empty") }
	if err := engine.RemoveTorrent(ctx, first, true); err != nil {
		t.Fatalf("engine removal reported as failed: %v", err)
	}
	if _, ok := engine.entries[first]; ok {
		t.Fatal("removed torrent is still active")
	}
	if _, ok := engine.removals[first]; !ok {
		t.Fatal("cleanup intent lost")
	}
	if err := engine.RemoveTorrent(ctx, first, true); err != nil {
		t.Fatal("removal is not idempotent", err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	restored := newTestEngine(t, state, root)
	defer restored.Close()
	items, err := restored.ListTorrents(ctx)
	if err != nil || len(items) != 1 || items[0].ID != second {
		t.Fatalf("restore lost the other torrent or resurrected removed one: %+v %v", items, err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatal("pending cleanup not retried after restart", err)
	}
	if len(restored.removals) != 0 {
		t.Fatal("completed cleanup not committed")
	}
}

func TestRemovalJournalWriteFailureLeavesEngineEntry(t *testing.T) {
	root := t.TempDir()
	engine := newTestEngine(t, filepath.Join(root, "state"), root)
	defer engine.Close()
	hash, err := engine.AddTorrent(context.Background(), "magnet:?xt=urn:btih:"+strings.Repeat("C", 40), filepath.Join(root, ".anidog-race", "task"))
	if err != nil {
		t.Fatal(err)
	}
	original := engine.manifestPath
	engine.manifestPath = filepath.Join(root, "directory")
	if err := os.Mkdir(engine.manifestPath, 0700); err != nil {
		t.Fatal(err)
	}
	err = engine.RemoveTorrent(context.Background(), hash, true)
	engine.manifestPath = original
	if err == nil || engine.entries[hash] == nil || len(engine.removals) != 0 {
		t.Fatal("failed intent write changed runtime state", err)
	}
}

func TestFailedRestoreDoesNotOverwriteRecoveryManifest(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	saved := manifest{Version: manifestVersion, Tasks: []manifestEntry{
		{InfoHash: strings.Repeat("D", 40), URL: "magnet:?xt=urn:btih:" + strings.Repeat("D", 40), SavePath: filepath.Join(root, ".anidog-race", "D")},
		{InfoHash: strings.Repeat("E", 40), URL: filepath.Join(root, "missing.torrent"), SavePath: filepath.Join(root, ".anidog-race", "E")},
	}, Removals: map[string]removalEntry{strings.Repeat("F", 40): {ContentPath: filepath.Join(root, ".anidog-race", "F"), SavePath: filepath.Join(root, ".anidog-race", "F"), RemoveFiles: true}}}
	raw, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "tasks.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	engine, err := New(&config.Config{BTStateDir: state, MediaRoot: root})
	if err == nil {
		engine.Close()
		t.Fatal("invalid restore unexpectedly succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("partial restore overwrote tasks or cleanup journal", err)
	}
}
