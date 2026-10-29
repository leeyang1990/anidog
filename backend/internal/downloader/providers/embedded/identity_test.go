package embedded

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
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
)

func versionedFixture(t *testing.T, root string, hybrid bool) (*metainfo.MetaInfo, string, []byte) {
	t.Helper()
	payload := make([]byte, 64<<10)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	leaves := make([][32]byte, 4)
	for i := range leaves {
		leaves[i] = sha256.Sum256(payload[i*(16<<10) : (i+1)*(16<<10)])
	}
	left := sha256.Sum256(append(leaves[0][:], leaves[1][:]...))
	right := sha256.Sum256(append(leaves[2][:], leaves[3][:]...))
	v2 := sha256.Sum256(append(left[:], right[:]...))
	private := true
	info := metainfo.Info{Name: "package", Private: &private, MetaVersion: 2, PieceLength: 32 << 10, FileTree: metainfo.FileTree{Dir: map[string]metainfo.FileTree{"payload.bin": {File: metainfo.FileTreeFile{Length: int64(len(payload)), PiecesRoot: string(v2[:])}}}}}
	if hybrid {
		first := sha1.Sum(payload[:32<<10])
		second := sha1.Sum(payload[32<<10:])
		info.Pieces = append(first[:], second[:]...)
		info.Files = []metainfo.FileInfo{{Length: int64(len(payload)), Path: []string{"payload.bin"}}}
	}
	raw, err := bencode.Marshal(&info)
	if err != nil {
		t.Fatal(err)
	}
	mi := &metainfo.MetaInfo{InfoBytes: raw, PieceLayers: map[string]string{string(v2[:]): string(append(left[:], right[:]...))}}
	path := filepath.Join(root, "fixture.torrent")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = mi.Write(f); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return mi, path, payload
}

func TestContentIdentityRejectsMissingAndPreservesFullAliases(t *testing.T) {
	for _, spec := range []*atorrent.TorrentSpec{nil, {}} {
		if _, err := contentIdentity(spec, ""); err == nil {
			t.Fatal("missing identity accepted")
		}
	}
	mi, _, _ := versionedFixture(t, t.TempDir(), true)
	spec := atorrent.TorrentSpecFromMetaInfo(mi)
	v1 := strings.ToUpper(spec.InfoHash.HexString())
	v2 := strings.ToUpper(spec.InfoHashV2.Value.HexString())
	for _, tc := range []struct{ retained, want string }{{"", v1}, {v1, v1}, {strings.ToLower(v2), v2}} {
		got, err := contentIdentity(spec, tc.retained)
		if err != nil || got != tc.want {
			t.Fatal(got, err, tc.want)
		}
	}
	for _, bad := range []string{strings.Repeat("F", 40), v2[:40]} {
		if _, err := contentIdentity(spec, bad); err == nil {
			t.Fatal("unproven or truncated identity accepted", bad)
		}
	}
	bad := *spec
	bad.InfoHashV2.Value = [32]byte{31: 1}
	if _, err := contentIdentity(&bad, ""); err == nil {
		t.Fatal("zero protocol key accepted")
	}
}

// A dependency panic must be observed as a child process failure instead of
// being hidden by a deferred Close trying to acquire its unreleased lock.
func TestEngineV2IdentityTransferAndRecovery(t *testing.T) {
	which := os.Getenv("ANIDOG_V2_IDENTITY_TEST")
	if which == "" {
		for _, mode := range []string{"classic", "mmap"} {
			for _, scenario := range []string{"file", "magnet", "hybrid-v2-magnet"} {
				t.Run(mode+"/"+scenario, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEngineV2IdentityTransferAndRecovery$", "-test.timeout=30s")
					for _, env := range os.Environ() {
						if !strings.HasPrefix(env, "ANIDOG_V2_IDENTITY_TEST=") && !strings.HasPrefix(env, "TORRENT_STORAGE_DEFAULT_FILE_IO=") {
							cmd.Env = append(cmd.Env, env)
						}
					}
					cmd.Env = append(cmd.Env, "ANIDOG_V2_IDENTITY_TEST="+scenario, "TORRENT_STORAGE_DEFAULT_FILE_IO="+mode)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("v2 lifecycle: %v\n%s", err, out)
					}
				})
			}
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := t.TempDir()
	mi, source, payload := versionedFixture(t, root, strings.HasPrefix(which, "hybrid"))
	seedDir := filepath.Join(root, "seed")
	if err := os.MkdirAll(filepath.Join(seedDir, "package"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "package", "payload.bin"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	seed := newTestEngine(t, filepath.Join(root, "seed-state"), filepath.Join(root, "seed-default"))
	defer seed.Close()
	if _, err := seed.AddTorrent(ctx, source, seedDir); err != nil {
		t.Fatal(err)
	}
	spec := atorrent.TorrentSpecFromMetaInfo(mi)
	want := strings.ToUpper(spec.InfoHashV2.Value.HexString())
	original := source
	if strings.Contains(which, "magnet") {
		original = "magnet:?xt=urn:btmh:1220" + want
	}
	stateDir, downloadDir := filepath.Join(root, "state"), filepath.Join(root, "download")
	engine := newTestEngine(t, stateDir, filepath.Join(root, "different-default"))
	receipt, err := engine.AddTorrent(ctx, original, downloadDir)
	if err != nil {
		t.Fatal(err)
	}
	if receipt != want || len(receipt) != 64 {
		t.Fatalf("full v2 receipt required: %s / %s", receipt, want)
	}
	engine.mu.RLock()
	torrent := engine.entries[receipt].torrent
	engine.mu.RUnlock()
	torrent.AddClientPeer(seed.client)
	waitFor(t, ctx, func() bool {
		rows, err := engine.ListTorrents(ctx)
		return err == nil && len(rows) == 1 && rows[0].ID == receipt && rows[0].Progress == 1 && rows[0].HasMetadata && rows[0].State == "uploading"
	})
	path := filepath.Join(downloadDir, "package", "payload.bin")
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatal("real v2 payload mismatch", err)
	}
	waitFor(t, ctx, func() bool { return fileExists(engine.metainfoPath(receipt)) })
	aliasServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = mi.Write(w) }))
	defer aliasServer.Close()
	alias, err := engine.AddTorrent(ctx, aliasServer.URL, downloadDir)
	if err != nil || alias != receipt {
		t.Fatal("metadata alias changed receipt", alias, err)
	}
	if err = engine.PauseTorrent(ctx, strings.ToLower(receipt)); err != nil {
		t.Fatal(err)
	}
	if err = engine.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(source); err != nil {
		t.Fatal(err)
	}
	restored := newTestEngine(t, stateDir, filepath.Join(root, "different-default"))
	defer restored.Close()
	rows, err := restored.ListTorrents(ctx)
	if err != nil || len(rows) != 1 || rows[0].ID != receipt || rows[0].State != "pausedDL" {
		t.Fatal("restored receipt or pause lost", rows, err)
	}
	raw, err := os.ReadFile(restored.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved manifest
	if err = json.Unmarshal(raw, &saved); err != nil || len(saved.Tasks) != 1 || saved.Tasks[0].InfoHash != receipt {
		t.Fatal("manifest identity changed", err)
	}
	if err = restored.ResumeTorrent(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	waitFor(t, ctx, func() bool {
		rows, _ := restored.ListTorrents(ctx)
		return len(rows) == 1 && rows[0].ID == receipt && rows[0].Progress == 1
	})
	if err = restored.RemoveTorrent(ctx, receipt, false); err != nil {
		t.Fatal(err)
	}
	rows, err = restored.ListTorrents(ctx)
	if err != nil || len(rows) != 0 {
		t.Fatal("removed receipt remains", rows, err)
	}
	got, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatal("removal changed retained bytes", err)
	}
}
