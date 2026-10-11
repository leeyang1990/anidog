package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanEpisodeFiles(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"Anime S03E01.mkv",
		"[Group] Anime S03E02.mp4",
		"[Group] Anime S02E03.mkv",
		"poster.jpg",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	present, count, err := scanEpisodeFiles(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("media file count = %d, want 3", count)
	}
	if !present[1] || !present[2] || present[3] {
		t.Fatalf("unexpected episode set: %#v", present)
	}
}

func TestLatestAiredEpisode(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.Local)
	airDates := map[int]string{1: "2026-07-01", 2: "2026-07-08", 3: "2026-07-15"}
	if got := latestAiredEpisode(3, airDates, now); got != 2 {
		t.Fatalf("latest aired = %d, want 2", got)
	}
}

func TestScanIgnoresStagingRejectedExtrasAndEmptyMedia(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"Anime S01E01.anidog-stream_123.part.mp4",
		"Anime S01E02.part.mp4",
		".anidog-rejected/Anime S01E03.mp4",
		"Extras/Anime S01E04.mkv",
		".anidog-race/Anime S01E05.mkv",
		"Anime S01E06.mkv",
	} {
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte("untrusted"), 0600)
	}
	os.WriteFile(filepath.Join(dir, "Anime S01E07.mp4"), nil, 0600)
	present, count, err := scanEpisodeFiles(dir, 1)
	if err != nil || count != 1 || len(present) != 1 || !present[6] {
		t.Fatalf("temporary media counted as complete: %v count=%d err=%v", present, count, err)
	}
}
