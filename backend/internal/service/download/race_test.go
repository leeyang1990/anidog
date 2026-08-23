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
	var savedEpisode model.AnimeEpisode
	if err := db.Where("anime_id = ? AND episode_number = ?", anime.ID, episode).
		First(&savedEpisode).Error; err != nil {
		t.Fatal(err)
	}
	if !savedEpisode.Downloaded || savedEpisode.FilePath == nil || *savedEpisode.FilePath != final {
		t.Fatalf("episode state was not synchronized: %#v", savedEpisode)
	}
	if torrentExec.removedID != torrentCandidate.TorrentID || torrentExec.removeFiles {
		t.Fatalf("torrent loser cleanup id=%q removeFiles=%t", torrentExec.removedID, torrentExec.removeFiles)
	}
}

func TestReconcileCompletedEpisodeStatesClosesDuplicatesAndRepairsFlag(t *testing.T) {
	db := testutil.InitTestDB()
	anime := model.Anime{Title: "historical drift"}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	episode := 5
	if err := db.Create(&model.AnimeEpisode{
		AnimeID: anime.ID, EpisodeNumber: episode, Downloaded: false,
	}).Error; err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(t.TempDir(), "historical S01E05.mkv")
	winner := model.Download{
		TorrentID: "winner", Name: "winner", URL: "https://example.test/winner",
		Status: model.DownloadStatusCompleted, DownloadType: model.DownloadTypeStream,
		AnimeID: &anime.ID, EpisodeNumber: &episode, FilePath: &final,
	}
	duplicate := model.Download{
		TorrentID: "duplicate", Name: "duplicate", URL: "magnet:?xt=urn:btih:DUPLICATE",
		Status: model.DownloadStatusCompleted, DownloadType: model.DownloadTypeTorrent,
		AnimeID: &anime.ID, EpisodeNumber: &episode,
	}
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&winner).Error; err != nil {
		t.Fatal(err)
	}

	service := &Service{db: db, executors: map[string]Executor{}}
	service.reconcileCompletedEpisodeStates(context.Background())

	var savedWinner, savedDuplicate model.Download
	_ = db.First(&savedWinner, winner.ID).Error
	_ = db.First(&savedDuplicate, duplicate.ID).Error
	if savedWinner.Status != model.DownloadStatusCompleted || savedDuplicate.Status != model.DownloadStatusSuperseded {
		t.Fatalf("winner=%s duplicate=%s", savedWinner.Status, savedDuplicate.Status)
	}
	var savedEpisode model.AnimeEpisode
	if err := db.Where("anime_id = ? AND episode_number = ?", anime.ID, episode).
		First(&savedEpisode).Error; err != nil {
		t.Fatal(err)
	}
	if !savedEpisode.Downloaded || savedEpisode.FilePath == nil || *savedEpisode.FilePath != final {
		t.Fatalf("episode drift was not repaired: %#v", savedEpisode)
	}
}

func TestExecutorTaskIDPrefersTorrentInfoHash(t *testing.T) {
	hash := "ABCDEF0123456789ABCDEF0123456789ABCDEF01"
	torrent := model.Download{
		TorrentID:    "torrent_temporary",
		DownloadType: model.DownloadTypeTorrent,
		InfoHash:     &hash,
	}
	if got := executorTaskID(&torrent); got != hash {
		t.Fatalf("torrent control id=%q want info hash", got)
	}
	stream := model.Download{TorrentID: "stream_runtime", DownloadType: model.DownloadTypeStream}
	if got := executorTaskID(&stream); got != stream.TorrentID {
		t.Fatalf("stream control id=%q want runtime id", got)
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

func TestCompleteEpisodeRaceRecoversAfterFileMovedBeforeDatabaseCommit(t *testing.T) {
	db := testutil.InitTestDB()
	anime := model.Anime{Title: "race recovery"}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	episode := 4
	candidate := model.Download{
		TorrentID: "hash-recovery", Name: "bt", URL: "magnet:?xt=urn:btih:RECOVERY",
		Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeTorrent,
		Source: SourceBT, AnimeID: &anime.ID, EpisodeNumber: &episode,
	}
	if err := db.Create(&candidate).Error; err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	staging := filepath.Join(dir, "missing-after-prior-rename.mkv")
	final := filepath.Join(dir, "episode.mkv")
	if err := os.WriteFile(final, []byte("already-promoted"), 0600); err != nil {
		t.Fatal(err)
	}
	service := &Service{
		db: db,
		executors: map[string]Executor{
			model.DownloadTypeTorrent: &recordingExecutor{},
		},
	}
	if !service.CompleteEpisodeRace(context.Background(), candidate.ID, staging, final) {
		t.Fatal("retry should reconcile a file that was already promoted")
	}
	var saved model.Download
	if err := db.First(&saved, candidate.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.DownloadStatusCompleted || saved.FilePath == nil || *saved.FilePath != final {
		t.Fatalf("candidate was not reconciled: %#v", saved)
	}
}
