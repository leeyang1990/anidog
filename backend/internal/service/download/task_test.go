package download

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestTaskValidate_ValidTorrent(t *testing.T) {
	task := &Task{
		Name:         "test torrent",
		URL:          "magnet:?xt=urn:btih:abc",
		DownloadType: model.DownloadTypeTorrent,
		Source:       SourceManual,
	}
	if err := task.Validate(); err != nil {
		t.Errorf("valid torrent task should pass: %v", err)
	}
}

func TestTaskValidate_ValidStream(t *testing.T) {
	task := &Task{
		Name:         "test stream",
		URL:          "https://example.com/ep1",
		DownloadType: model.DownloadTypeStream,
		Source:       SourceStream,
		StreamRule:   &model.StreamRule{Name: "rule1"},
	}
	if err := task.Validate(); err != nil {
		t.Errorf("valid stream task should pass: %v", err)
	}
}

func TestTaskValidate_MissingName(t *testing.T) {
	task := &Task{
		URL:          "magnet:?xt=urn:btih:abc",
		DownloadType: model.DownloadTypeTorrent,
		Source:       SourceManual,
	}
	if err := task.Validate(); err == nil {
		t.Error("missing name should fail validation")
	}
}

func TestTaskValidate_MissingURL(t *testing.T) {
	task := &Task{
		Name:         "test",
		DownloadType: model.DownloadTypeTorrent,
		Source:       SourceManual,
	}
	if err := task.Validate(); err == nil {
		t.Error("missing URL should fail validation")
	}
}

func TestTaskValidate_InvalidDownloadType(t *testing.T) {
	task := &Task{
		Name:         "test",
		URL:          "https://example.com",
		DownloadType: "ftp",
		Source:       SourceManual,
	}
	if err := task.Validate(); err == nil {
		t.Error("invalid download type should fail validation")
	}
}

func TestTaskValidate_InvalidSource(t *testing.T) {
	task := &Task{
		Name:         "test",
		URL:          "magnet:?xt=urn:btih:abc",
		DownloadType: model.DownloadTypeTorrent,
		Source:       "unknown",
	}
	if err := task.Validate(); err == nil {
		t.Error("invalid source should fail validation")
	}
}

func TestTaskValidate_StreamMissingRule(t *testing.T) {
	task := &Task{
		Name:         "test stream",
		URL:          "https://example.com/ep1",
		DownloadType: model.DownloadTypeStream,
		Source:       SourceStream,
	}
	if err := task.Validate(); err == nil {
		t.Error("stream task without StreamRule should fail validation")
	}
}

func TestTaskValidate_AllSources(t *testing.T) {
	sources := []string{SourceManual, SourceStream, SourceBangumi, SourceRSS, SourceBT, SourceMikan}
	for _, src := range sources {
		task := &Task{
			Name:         "test",
			URL:          "magnet:?xt=urn:btih:abc",
			DownloadType: model.DownloadTypeTorrent,
			Source:       src,
		}
		if err := task.Validate(); err != nil {
			t.Errorf("source %q should be valid: %v", src, err)
		}
	}
}

func TestGenerateTorrentID(t *testing.T) {
	id1 := generateTorrentID(model.DownloadTypeTorrent)
	id2 := generateTorrentID(model.DownloadTypeStream)
	id3 := generateTorrentID("other")

	if len(id1) < 8 {
		t.Errorf("torrent ID too short: %s", id1)
	}
	if id1[:8] != "torrent_" {
		t.Errorf("torrent ID prefix wrong: %s", id1)
	}
	if id2[:7] != "stream_" {
		t.Errorf("stream ID prefix wrong: %s", id2)
	}
	if id3 == id1 || id3 == id2 {
		t.Error("IDs should be unique")
	}
}

func TestResolveSavePath_TaskPath(t *testing.T) {
	task := &Task{SavePath: "/custom/path"}
	result := resolveSavePath(&config.Config{}, task)
	if result == nil || *result != "/custom/path" {
		t.Errorf("expected /custom/path, got %v", result)
	}
}

func TestResolveSavePath_StreamDefault(t *testing.T) {
	cfg := config.Config{StreamDownloadDir: "/stream/dir"}
	task := &Task{DownloadType: model.DownloadTypeStream}
	result := resolveSavePath(&cfg, task)
	if result == nil || *result != "/stream/dir" {
		t.Errorf("expected /stream/dir, got %v", result)
	}
}

func TestResolveSavePath_MediaRoot(t *testing.T) {
	cfg := config.Config{MediaRoot: "/media"}
	task := &Task{DownloadType: model.DownloadTypeTorrent}
	result := resolveSavePath(&cfg, task)
	if result == nil || *result != "/media/torrent" {
		t.Errorf("expected /media/torrent, got %v", result)
	}
}

func TestResolveSavePath_Nil(t *testing.T) {
	task := &Task{DownloadType: model.DownloadTypeTorrent}
	result := resolveSavePath(&config.Config{}, task)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestResolveTaskSavePathUsesRuntimeDownloadDirForAnimeTorrent(t *testing.T) {
	db := testutil.InitTestDB()
	if err := db.Create(&model.Setting{Key: "download_dir", Value: "/downloads/anime"}).Error; err != nil {
		t.Fatal(err)
	}
	animeID, episode := uint(42), 3
	task := &Task{
		Name:          "manual anime torrent",
		URL:           "magnet:?xt=urn:btih:ABCDEF0123456789ABCDEF0123456789ABCDEF01",
		DownloadType:  model.DownloadTypeTorrent,
		Source:        SourceBT,
		AnimeID:       &animeID,
		EpisodeNumber: &episode,
	}
	service := NewService(db, &config.Config{MediaRoot: "/wrong-fallback"}, nil)
	got := service.resolveTaskSavePath(context.Background(), task)
	if got == nil || !IsTorrentRaceSavePath(*got) {
		t.Fatalf("expected isolated torrent path, got %v", got)
	}
	wantPrefix := filepath.Join("/downloads/anime", torrentRaceDir, "anime-42", "episode-003")
	if rel, err := filepath.Rel(wantPrefix, *got); err != nil || rel == ".." || filepath.IsAbs(rel) {
		t.Fatalf("path %q is not under %q", *got, wantPrefix)
	}
}

func TestResolveTaskSavePathPreservesExplicitPath(t *testing.T) {
	db := testutil.InitTestDB()
	animeID, episode := uint(1), 1
	task := &Task{
		DownloadType:  model.DownloadTypeTorrent,
		AnimeID:       &animeID,
		EpisodeNumber: &episode,
		SavePath:      "/custom/path",
	}
	service := NewService(db, &config.Config{}, nil)
	got := service.resolveTaskSavePath(context.Background(), task)
	if got == nil || *got != task.SavePath {
		t.Fatalf("explicit path changed: %v", got)
	}
}

func TestEnsureMediaCapacityRejectsInsufficientFreeSpace(t *testing.T) {
	db := testutil.InitTestDB()
	root := t.TempDir()
	if err := db.Create(&model.Setting{Key: "media.min_free_gb", Value: "999999999"}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(db, &config.Config{MediaRoot: root}, nil)
	if err := service.ensureMediaCapacity(context.Background()); err == nil {
		t.Fatal("expected capacity guard to reject an impossible free-space requirement")
	}
}

func TestEnsureMediaCapacityIsDisabledWithoutThresholds(t *testing.T) {
	db := testutil.InitTestDB()
	service := NewService(db, &config.Config{MediaRoot: "/path/that/does/not/exist"}, nil)
	if err := service.ensureMediaCapacity(context.Background()); err != nil {
		t.Fatalf("capacity guard should be opt-in: %v", err)
	}
}
