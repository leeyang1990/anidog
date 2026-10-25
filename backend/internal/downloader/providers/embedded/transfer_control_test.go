package embedded

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	atorrent "github.com/anacrolix/torrent"
	"github.com/anidog/anidog-go/internal/downloader"
)

func TestTransferControlKeepsPeerSchedulingConsistent(t *testing.T) {
	if os.Getenv("ANIDOG_TRANSFER_CONTROL_TEST") != "1" {
		for _, mode := range []string{"classic", "mmap"} {
			t.Run(mode, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTransferControlKeepsPeerSchedulingConsistent$")
				for _, value := range os.Environ() {
					if !strings.HasPrefix(value, "TORRENT_STORAGE_DEFAULT_FILE_IO=") && !strings.HasPrefix(value, "ANIDOG_TRANSFER_CONTROL_TEST=") {
						cmd.Env = append(cmd.Env, value)
					}
				}
				cmd.Env = append(cmd.Env, "TORRENT_STORAGE_DEFAULT_FILE_IO="+mode, "ANIDOG_TRANSFER_CONTROL_TEST=1")
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("transfer scheduling with %s IO: %v\n%s", mode, err, output)
				}
			})
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := t.TempDir()
	engine := newTestEngine(t, filepath.Join(root, "state"), filepath.Join(root, "default"))
	defer engine.Close()
	policy := downloader.TorrentPolicy{MaxActiveDownloads: 2}
	if err := engine.SetPolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	var hashes []string
	var torrents []*atorrent.Torrent
	for _, name := range []string{"first", "second"} {
		payload := make([]byte, 128<<10)
		if _, err := rand.Read(payload); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(root, name+".bin")
		if err := os.WriteFile(source, payload, 0600); err != nil {
			t.Fatal(err)
		}
		metadataDir := filepath.Join(root, name+"-metadata")
		if err := os.Mkdir(metadataDir, 0700); err != nil {
			t.Fatal(err)
		}
		hash, err := engine.AddTorrent(ctx, buildTorrentFile(t, source, metadataDir), filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, hash)
		engine.mu.RLock()
		torrents = append(torrents, engine.entries[hash].torrent)
		engine.mu.RUnlock()
	}
	waitWanted := func(torrent *atorrent.Torrent) {
		waitFor(t, ctx, func() bool {
			for _, piece := range torrent.PieceStateRuns() {
				if !piece.Ok || piece.Hashing || piece.QueuedForHash || piece.Marking || piece.Priority != atorrent.PiecePriorityNormal {
					return false
				}
			}
			return true
		})
	}
	for _, torrent := range torrents {
		waitWanted(torrent)
	}
	probePeerScheduling := func(torrent *atorrent.Torrent) {
		// Exercise the public connection scheduler while suspended. A fresh
		// subprocess keeps upstream's sampled invariants from being amortized
		// away by unrelated tests. No external peer or network wait is needed.
		for range 256 {
			torrent.SetMaxEstablishedConns(0)
		}
		if torrent.BytesCompleted() != 0 {
			t.Fatal("fixture transferred data without a peer")
		}
	}
	if err := engine.PauseTorrent(ctx, hashes[0]); err != nil {
		t.Fatal(err)
	}
	probePeerScheduling(torrents[0])
	if err := engine.ResumeTorrent(ctx, hashes[0]); err != nil {
		t.Fatal(err)
	}
	waitWanted(torrents[0])
	policy.MaxActiveDownloads = 1
	if err := engine.SetPolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	probePeerScheduling(torrents[1])
	policy.MaxActiveDownloads = 2
	if err := engine.SetPolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	waitWanted(torrents[1])
}
