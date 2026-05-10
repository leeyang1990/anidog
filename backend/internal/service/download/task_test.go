package download

import (
	"testing"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
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
	sources := []string{SourceManual, SourceStream, SourceBangumi, SourceRSS}
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
