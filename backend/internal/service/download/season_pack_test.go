package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestMapSeasonPackMediaFilesMapsEpisodesAndOVA(t *testing.T) {
	dir := t.TempDir()
	for ep := 1; ep <= 4; ep++ {
		path := filepath.Join(dir, fmt.Sprintf("[Group][Lucky Star][%02d][1080p].mkv", ep))
		if err := os.WriteFile(path, make([]byte, ep), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ova := filepath.Join(dir, "[Group][Lucky Star][OVA][1080p].mkv")
	if err := os.WriteFile(ova, make([]byte, 5), 0600); err != nil {
		t.Fatal(err)
	}
	mapped, err := mapSeasonPackMediaFiles(dir, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapped) != 5 || mapped[5] != ova {
		t.Fatalf("mapped=%v, want episodes 1-5 with OVA at 5", mapped)
	}
}

func TestCompleteSeasonPackArchivesEveryMappedEpisode(t *testing.T) {
	db := testutil.InitTestDB()
	year, season := 2007, 1
	anime := model.Anime{Title: "Lucky Star", Year: &year, Season: &season, IsSubscribed: true}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	mediaRoot := t.TempDir()
	start, end, trigger := 1, 4, 1
	savePath := BuildTorrentPackSavePath(mediaRoot, anime.ID, start, end, "HASH", "magnet")
	if err := os.MkdirAll(savePath, 0755); err != nil {
		t.Fatal(err)
	}
	files := make(map[int]string)
	for ep := start; ep <= end; ep++ {
		path := filepath.Join(savePath, fmt.Sprintf("episode-%02d.mkv", ep))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("episode-%d", ep)), 0600); err != nil {
			t.Fatal(err)
		}
		files[ep] = path
	}
	dl := model.Download{
		TorrentID: "pack", Name: "Lucky Star - 合集 01-04", URL: "magnet",
		SavePath: &savePath, Status: model.DownloadStatusDownloading,
		DownloadType: model.DownloadTypeTorrent, Source: SourceMikan,
		AnimeID: &anime.ID, EpisodeNumber: &trigger, Scope: model.DownloadScopeSeason,
		EpisodeStart: &start, EpisodeEnd: &end,
	}
	if err := db.Create(&dl).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, &config.Config{MediaRoot: mediaRoot}, nil)
	if !svc.CompleteSeasonPack(context.Background(), dl.ID, files, mediaRoot) {
		t.Fatal("season pack completion failed")
	}
	var refreshed model.Download
	if err := db.First(&refreshed, dl.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != model.DownloadStatusCompleted {
		t.Fatalf("status=%q, want completed", refreshed.Status)
	}
	var episodes []model.AnimeEpisode
	db.Where("anime_id = ? AND downloaded = ?", anime.ID, true).Order("episode_number").Find(&episodes)
	if len(episodes) != 4 {
		t.Fatalf("archived %d episodes, want 4", len(episodes))
	}
	for _, episode := range episodes {
		if episode.FilePath == nil {
			t.Fatalf("episode %d has no file path", episode.EpisodeNumber)
		}
		if _, err := os.Stat(*episode.FilePath); err != nil {
			t.Fatalf("episode %d final file missing: %v", episode.EpisodeNumber, err)
		}
	}
}

func TestMapSeasonPackMediaFilesRejectsOpaqueLowCoverage(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 8; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("video-%c.mkv", 'a'+i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := mapSeasonPackMediaFiles(dir, 1, 12); err == nil {
		t.Fatal("opaque pack must not be archived by guessed ordering")
	}
}
