package embedded

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	atorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
	"github.com/anacrolix/torrent/storage"
)

// Exercise the dependency's public registry, tracker and TCP interfaces in fresh
// processes: a panic holding its client lock must fail instead of hanging cleanup.
func TestTrackerIdentityLifecycle(t *testing.T) {
	which := os.Getenv("ANIDOG_TRACKER_LIFECYCLE")
	if which == "" {
		for _, mode := range []string{"classic", "mmap"} {
			for _, scenario := range []string{"metadata", "magnet", "readd"} {
				t.Run(mode+"/"+scenario, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTrackerIdentityLifecycle$", "-test.timeout=30s")
					for _, value := range os.Environ() {
						if !strings.HasPrefix(value, "TORRENT_STORAGE_DEFAULT_FILE_IO=") && !strings.HasPrefix(value, "ANIDOG_TRACKER_LIFECYCLE=") {
							cmd.Env = append(cmd.Env, value)
						}
					}
					cmd.Env = append(cmd.Env, "TORRENT_STORAGE_DEFAULT_FILE_IO="+mode, "ANIDOG_TRACKER_LIFECYCLE="+scenario)
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("tracker lifecycle: %v\n%s", err, output)
					}
				})
			}
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := t.TempDir()
	cfg := atorrent.NewDefaultClientConfig()
	cfg.DataDir = filepath.Join(root, "default")
	cfg.ListenHost = func(network string) string {
		if strings.HasSuffix(network, "6") {
			return "::1"
		}
		return "127.0.0.1"
	}
	cfg.ListenPort = 0
	cfg.DisableAcceptRateLimiting = true
	cfg.NoDHT, cfg.NoDefaultPortForwarding, cfg.DisablePEX = true, true, true
	cl, err := atorrent.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 16<<10)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	h1, h2 := sha1.Sum(payload), sha256.Sum256(payload)
	info := metainfo.Info{Name: "fixture.bin", Length: int64(len(payload)), PieceLength: 16 << 10, Pieces: h1[:], MetaVersion: 2,
		FileTree: metainfo.FileTree{Dir: map[string]metainfo.FileTree{"fixture.bin": {File: metainfo.FileTreeFile{Length: int64(len(payload)), PiecesRoot: string(h2[:])}}}}}
	infoBytes, err := bencode.Marshal(&info)
	if err != nil {
		t.Fatal(err)
	}
	mi := &metainfo.MetaInfo{InfoBytes: infoBytes}
	spec := atorrent.TorrentSpecFromMetaInfo(mi)
	v1, v2 := spec.InfoHash, *spec.InfoHashV2.Value.ToShort()
	requests := make(chan string, 32)
	release := make(chan struct{})
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash := r.URL.Query().Get("info_hash")
		select {
		case requests <- hash:
		default:
		}
		if which == "readd" {
			select {
			case <-release:
			case <-ctx.Done():
				return
			}
		}
		body, _ := bencode.Marshal(map[string]any{"interval": 1, "peers": string([]byte{127, 0, 0, 1, 0, 1})})
		_, _ = w.Write(body)
	}))
	defer tracker.Close()
	spec.Trackers = [][]string{{tracker.URL + "/announce"}}
	if which == "magnet" {
		spec.InfoBytes = nil
	}
	torrent, _, err := cl.AddTorrentSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if which == "readd" {
		select {
		case <-requests:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		torrent.Drop()
		for _, hash := range []metainfo.Hash{v1, v2} {
			if _, ok := cl.Torrent(hash); ok {
				t.Fatal("dropped alias remains registered")
			}
		}
		store := storage.NewFile(filepath.Join(root, "replacement"))
		spec.Storage = store
		replacement, fresh, err := cl.AddTorrentSpec(spec)
		if err != nil || !fresh || replacement == torrent {
			t.Fatalf("readd: %v %v", fresh, err)
		}
		close(release)
		// A tracker response belongs to the generation that issued it. Subsequent
		// requests must resolve the live registry, not the old weak pointer.
		waitFor(t, ctx, func() bool { return replacement.Stats().TotalPeers > 0 })
		if torrent.Stats().TotalPeers != 0 {
			t.Fatal("late response populated dropped torrent")
		}
		torrent = replacement
		t.Cleanup(func() { _ = store.Close() })
	} else {
		seen := map[string]bool{}
		for !seen[string(v1[:])] || !seen[string(v2[:])] {
			select {
			case hash := <-requests:
				seen[hash] = true
			case <-ctx.Done():
				t.Fatal("both identities did not announce", ctx.Err())
			}
		}
	}
	for _, hash := range []metainfo.Hash{v1, v2} {
		if owner, ok := cl.Torrent(hash); !ok || owner != torrent {
			t.Fatalf("alias %x has wrong owner", hash)
		}
		if which != "magnet" {
			setDownloadDemand(torrent, true)
			waitFor(t, ctx, func() bool {
				for _, p := range torrent.PieceStateRuns() {
					if !p.Ok || p.Hashing || p.QueuedForHash || p.Marking || p.Priority != atorrent.PiecePriorityNormal {
						return false
					}
				}
				return true
			})
		}
		conn, err := net.DialTimeout("tcp", cl.ListenAddrs()[0].String(), 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, err = pp.Handshake(ctx, conn, &hash, sha1.Sum([]byte(fmt.Sprint(hash))), pp.NewPeerExtensionBytes())
		if err != nil {
			_ = conn.Close()
			t.Fatalf("handshake for %x (v1 %x / v2 %x): %v", hash, v1, v2, err)
		}
		go io.Copy(io.Discard, conn)
		_ = conn.Close()
	}
	torrent.Drop()
	// A late AddTrackers call is permitted after Drop; it cannot install an
	// unregistered dispatcher key, even when a new URL is supplied.
	torrent.AddTrackers([][]string{{tracker.URL + "/late"}})
	cl.Close()
}
