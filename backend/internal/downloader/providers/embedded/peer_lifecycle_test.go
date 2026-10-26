package embedded

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	atorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
)

// A peer changing its advertised inventory must not leave availability behind
// after removal. Exercise the real decoder and lifecycle, without external peers.
func TestPeerInventoryChangesDoNotCrashRemoval(t *testing.T) {
	if os.Getenv("ANIDOG_PEER_LIFECYCLE_TEST") == "" {
		for _, mode := range []string{"classic", "mmap"} {
			for _, action := range []string{"remove", "close"} {
				for _, inventory := range []string{"all-none", "partial-none", "repeated-all-none"} {
					t.Run(mode+"/"+action+"/"+inventory, func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPeerInventoryChangesDoNotCrashRemoval$")
						for _, value := range os.Environ() {
							if !strings.HasPrefix(value, "TORRENT_STORAGE_DEFAULT_FILE_IO=") && !strings.HasPrefix(value, "ANIDOG_PEER_LIFECYCLE_") {
								cmd.Env = append(cmd.Env, value)
							}
						}
						cmd.Env = append(cmd.Env, "TORRENT_STORAGE_DEFAULT_FILE_IO="+mode, "ANIDOG_PEER_LIFECYCLE_TEST="+action, "ANIDOG_PEER_LIFECYCLE_INVENTORY="+inventory)
						if output, err := cmd.CombinedOutput(); err != nil {
							t.Fatalf("peer lifecycle: %v\n%s", err, output)
						}
					})
				}
			}
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "payload.bin")
	payload := make([]byte, 128<<10)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	engine := newTestEngine(t, filepath.Join(root, "state"), filepath.Join(root, "default"))
	// This child owns the engine. Close explicitly after the lifecycle check:
	// deferring Close while a dependency panic holds its lock hides the panic
	// behind a cleanup deadlock instead of reporting the failing contract.
	hash, err := engine.AddTorrent(ctx, buildTorrentFile(t, source, root), filepath.Join(root, "download"))
	if err != nil {
		t.Fatal(err)
	}
	engine.mu.RLock()
	torrent := engine.entries[hash].torrent
	engine.mu.RUnlock()
	// The library rejects incoming connections while its initial disk hash
	// has not yet published any wanted pieces. Wait on that observable state
	// rather than racing the handshake against initialization.
	waitFor(t, ctx, func() bool {
		for _, state := range torrent.PieceStateRuns() {
			if !state.Ok || state.Hashing || state.QueuedForHash || state.Marking || state.Priority != atorrent.PiecePriorityNormal {
				return false
			}
		}
		return true
	})
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", engine.listenPort), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	infoHash := metainfo.NewHashFromHex(hash)
	if _, err := pp.Handshake(ctx, conn, &infoHash, [20]byte{1}, pp.NewPeerExtensionBytes(pp.ExtensionBitFast)); err != nil {
		t.Fatal(err)
	}
	go io.Copy(io.Discard, conn)
	messages := []pp.Message{{Type: pp.HaveAll}, {Type: pp.HaveNone}}
	switch os.Getenv("ANIDOG_PEER_LIFECYCLE_INVENTORY") {
	case "partial-none":
		messages[0] = pp.Message{Type: pp.Bitfield, Bitfield: []bool{false, true, false, false, false, false, false, false}}
	case "repeated-all-none":
		messages = append(messages, messages...)
	}
	messages = append(messages, pp.Message{Type: pp.Have, Index: 0})
	for _, msg := range messages {
		if _, err := conn.Write(msg.MustMarshalBinary()); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, ctx, func() bool {
		for _, peer := range torrent.PeerConns() {
			if pieces := peer.PeerPieces(); pieces.GetCardinality() == 1 && pieces.Contains(0) {
				return true
			}
		}
		return false
	})
	if os.Getenv("ANIDOG_PEER_LIFECYCLE_TEST") == "remove" {
		if err := engine.RemoveTorrent(ctx, hash, false); err != nil {
			t.Fatal(err)
		}
		if rows, err := engine.ListTorrents(ctx); err != nil || len(rows) != 0 {
			t.Fatalf("removed torrent retained: %+v %v", rows, err)
		}
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
}
