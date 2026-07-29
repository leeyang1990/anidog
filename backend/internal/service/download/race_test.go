package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestCompleteEpisodeRacePromotesFirstStreamAndCancelsTorrent(t *testing.T) {
	db := testutil.InitTestDB()
	anime := model.Anime{Title: "race anime"}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	episode := 2
	streamCandidate := model.Download{
		TorrentID: "stream-1", Name: "stream", URL: "https://example.test/2",
		Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeStream,
		Source: SourceStream, AnimeID: &anime.ID, EpisodeNumber: &episode,
	}
	torrentCandidate := model.Download{
		TorrentID: "hash-2", Name: "mikan", URL: "magnet:?xt=urn:btih:HASH2",
		Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeTorrent,
		Source: SourceMikan, AnimeID: &anime.ID, EpisodeNumber: &episode,
	}
	if err := db.Create(&streamCandidate).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&torrentCandidate).Error; err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	staging := filepath.Join(dir, "episode.part.mp4")
	final := filepath.Join(dir, "episode.mp4")
	if err := os.WriteFile(staging, []byte("valid-video-placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	torrentExec := &recordingExecutor{}
	service := &Service{
		db: db,
		executors: map[string]Executor{
			model.DownloadTypeStream:  &recordingExecutor{},
			model.DownloadTypeTorrent: torrentExec,
		},
	}
	if !service.CompleteEpisodeRace(context.Background(), streamCandidate.ID, staging, final) {
		t.Fatal("first completed candidate should win")
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging file still exists: %v", err)
	}
	content, err := os.ReadFile(final)
	if err != nil || string(content) != "valid-video-placeholder" {
		t.Fatalf("final file was not atomically promoted: %q err=%v", content, err)
	}
	var winner, loser model.Download
	_ = db.First(&winner, streamCandidate.ID).Error
	_ = db.First(&loser, torrentCandidate.ID).Error
	if winner.Status != model.DownloadStatusCompleted || loser.Status != model.DownloadStatusSuperseded {
		t.Fatalf("winner=%s loser=%s", winner.Status, loser.Status)
	}
	if torrentExec.removedID != torrentCandidate.TorrentID || torrentExec.removeFiles {
		t.Fatalf("torrent loser cleanup id=%q removeFiles=%t", torrentExec.removedID, torrentExec.removeFiles)
	}
}

func TestCompleteEpisodeRaceRejectsLateStreamCandidate(t *testing.T) {
	db := testutil.InitTestDB()
	anime := model.Anime{Title: "race anime"}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	episode := 3
	winner := model.Download{
		TorrentID: "hash-winner", Name: "bt", URL: "magnet:?xt=urn:btih:WIN",
		Status: model.DownloadStatusCompleted, DownloadType: model.DownloadTypeTorrent,
		Source: SourceBT, AnimeID: &anime.ID, EpisodeNumber: &episode,
	}
	late := model.Download{
		TorrentID: "stream-late", Name: "stream", URL: "https://example.test/3",
		Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeStream,
		Source: SourceStream, AnimeID: &anime.ID, EpisodeNumber: &episode,
	}
	if err := db.Create(&winner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&late).Error; err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(t.TempDir(), "late.part.mp4")
	if err := os.WriteFile(staging, []byte("late"), 0600); err != nil {
		t.Fatal(err)
	}
	streamExec := &recordingExecutor{}
	service := &Service{
		db: db,
		executors: map[string]Executor{
			model.DownloadTypeStream: streamExec,
		},
	}
	if service.CompleteEpisodeRace(context.Background(), late.ID, staging, filepath.Join(t.TempDir(), "final.mp4")) {
		t.Fatal("late candidate must not replace an existing winner")
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("late staging file was not removed: %v", err)
	}
	var saved model.Download
	_ = db.First(&saved, late.ID).Error
	if saved.Status != model.DownloadStatusSuperseded {
		t.Fatalf("late candidate status=%s", saved.Status)
	}
}
