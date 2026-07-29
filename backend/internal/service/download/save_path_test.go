package download

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTorrentRaceSavePathIsIsolatedAndReversible(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Anime")
	path := BuildTorrentRaceSavePath(root, 42, 3, strings.Repeat("a", 40), "")
	if !IsTorrentRaceSavePath(path) {
		t.Fatalf("path is not recognized as race staging: %s", path)
	}
	gotRoot, ok := TorrentRaceMediaRoot(path)
	if !ok || gotRoot != root {
		t.Fatalf("root=%q ok=%t, want %q", gotRoot, ok, root)
	}
	if !strings.Contains(path, filepath.Join(".anidog-race", "anime-42", "episode-003")) {
		t.Fatalf("unexpected staging layout: %s", path)
	}
}

func TestTorrentRaceSavePathFallsBackToStableURLKey(t *testing.T) {
	first := BuildTorrentRaceSavePath("/downloads/Anime", 7, 2, "", "https://example.test/a.torrent")
	second := BuildTorrentRaceSavePath("/downloads/Anime", 7, 2, "", "https://example.test/a.torrent")
	other := BuildTorrentRaceSavePath("/downloads/Anime", 7, 2, "", "https://example.test/b.torrent")
	if first != second || first == other {
		t.Fatalf("URL fallback key is not stable: %q %q %q", first, second, other)
	}
}
