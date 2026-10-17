package embedded

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// Run the same binary in separate processes with mmap and classic IO: the
// library selects IO in init, before a test can change its environment.
func TestSMBCompletedTorrentDirectoryCanMove(t *testing.T) {
	base := os.Getenv("ANIDOG_SMB_TEST_ROOT")
	if base == "" {
		t.Skip("requires an isolated directory on an SMB mount")
	}
	root, err := os.MkdirTemp(base, ".anidog-storage-check-")
	if err != nil {
		t.Fatal(err)
	}
	pinned := os.Getenv("ANIDOG_EXPECT_PINNED") == "1"
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil && !pinned {
			t.Error(err)
		}
	})
	dir := filepath.Join(root, "incoming")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "episode.bin"), make([]byte, 32768), 0600); err != nil {
		t.Fatal(err)
	}
	client := storage.NewFile(root)
	defer client.Close()
	info := metainfo.Info{Name: "incoming", Files: []metainfo.FileInfo{{Length: 32768, Path: []string{"episode.bin"}}}, PieceLength: 16384, Pieces: make([]byte, 40)}
	store, err := client.OpenTorrent(context.Background(), &info, metainfo.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Piece(info.Piece(0)).ReadAt(make([]byte, 1024), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err = os.Rename(dir, filepath.Join(root, "active"))
	if pinned {
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("expected the old mmap handle to block rename, got %v", err)
		}
		t.Log("reproduced: mmap storage retains file handles after torrent storage Close")
	} else if err != nil {
		t.Fatalf("completed directory remains pinned: %v", err)
	}
}
