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

func TestMapSeasonPackMediaFilesKeepsOVAApart(t *testing.T) {
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
	if len(mapped) != 4 || mapped[5] != "" {
		t.Fatalf("mapped=%v, want only regular episodes 1-4", mapped)
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
	contentDir := filepath.Join(savePath, "Lucky Star Pack")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatal(err)
	}
	files := make(map[int]string)
	for ep := start; ep <= end; ep++ {
		path := filepath.Join(contentDir, fmt.Sprintf("episode-%02d.mkv", ep))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("episode-%d", ep)), 0600); err != nil {
			t.Fatal(err)
		}
		files[ep] = path
	}
	if err := os.WriteFile(filepath.Join(contentDir, "episode-01.sc.ass"), []byte("subtitle"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(contentDir, "SP"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "SP", "bonus.mkv"), []byte("bonus"), 0600); err != nil {
		t.Fatal(err)
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
	seasonDir := filepath.Dir(*episodes[0].FilePath)
	if _, err := os.Stat(filepath.Join(seasonDir, "Lucky Star S01E01.sc.ass")); err != nil {
		t.Fatalf("episode sidecar was not archived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(seasonDir, "Extras", "Lucky Star Pack", "SP", "bonus.mkv")); err != nil {
		t.Fatalf("pack extras were not preserved: %v", err)
	}
	if _, err := os.Stat(savePath); !os.IsNotExist(err) {
		t.Fatalf("race directory was not cleaned: %v", err)
	}
	// A restart must not copy the pack's directory and total size onto EP1.
	svc.reconcileCompletedEpisodeStates(context.Background())
	var first model.AnimeEpisode
	if err := db.Where("anime_id = ? AND episode_number = 1", anime.ID).First(&first).Error; err != nil {
		t.Fatal(err)
	}
	if first.FilePath == nil || *first.FilePath != *episodes[0].FilePath || first.FileSize == nil || *first.FileSize != *episodes[0].FileSize {
		t.Fatalf("restart corrupted first episode: %+v", first)
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

func TestBackfillPackPreservesExistingEpisodeAndImportsMissing(t *testing.T) {
	for _, oldExt := range []string{".mkv", ".mp4"} {
		t.Run(oldExt, func(t *testing.T) {
			db := testutil.InitTestDB()
			root := t.TempDir()
			a := model.Anime{Title: "Backfill"}
			db.Create(&a)
			dir := BuildAnimeSavePath(root, &a)
			os.MkdirAll(dir, 0755)
			oldPath := filepath.Join(dir, "Backfill S01E01"+oldExt)
			os.WriteFile(oldPath, []byte("original verified file"), 0600)
			oldID := "original-winner"
			db.Create(&model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: 1, Downloaded: true, FilePath: &oldPath, DownloadID: &oldID})
			start, end, trigger := 1, 4, 2
			staging := BuildTorrentPackSavePath(root, a.ID, start, end, "PACK", "magnet")
			os.MkdirAll(staging, 0755)
			files := map[int]string{}
			for ep := 1; ep <= 4; ep++ {
				p := filepath.Join(staging, fmt.Sprintf("episode-%02d.mkv", ep))
				os.WriteFile(p, []byte("replacement candidate"), 0600)
				files[ep] = p
			}
			d := model.Download{TorrentID: "backfill-pack", Name: "pack", URL: "magnet", Status: model.DownloadStatusDownloading, AnimeID: &a.ID, EpisodeNumber: &trigger, Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end, SavePath: &staging}
			db.Create(&d)
			s := NewService(db, &config.Config{MediaRoot: root}, nil)
			if !s.CompleteSeasonPack(context.Background(), d.ID, files, root) {
				t.Fatal("backfill conflicted with an already archived episode")
			}
			var first model.AnimeEpisode
			db.Where("anime_id = ? AND episode_number = 1", a.ID).First(&first)
			if first.DownloadID == nil || *first.DownloadID != oldID || *first.FilePath != oldPath {
				t.Fatalf("existing mapping overwritten: %+v", first)
			}
			if content, _ := os.ReadFile(oldPath); string(content) != "original verified file" {
				t.Fatal("existing media overwritten")
			}
			var count int64
			db.Model(&model.AnimeEpisode{}).Where("anime_id = ? AND downloaded = ?", a.ID, true).Count(&count)
			if count != 4 {
				t.Fatalf("only %d episodes covered", count)
			}
			db.First(&d, d.ID)
			if d.QualityNote != "合集已归档 3 集" {
				t.Fatalf("incorrect backfill count: %s", d.QualityNote)
			}
		})
	}
}

func TestCoveredOrRejectedPackCannotResurrectMedia(t *testing.T) {
	db := testutil.InitTestDB()
	root := t.TempDir()
	a := model.Anime{Title: "Covered"}
	db.Create(&a)
	dir := BuildAnimeSavePath(root, &a)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "Covered S01E01.mkv")
	os.WriteFile(path, []byte("original"), 0600)
	db.Create(&model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: 1, Downloaded: true, FilePath: &path})
	d := model.Download{TorrentID: "covered-pack", Name: "pack", URL: "magnet", Status: model.DownloadStatusDownloading, AnimeID: &a.ID, Scope: model.DownloadScopeSeason}
	db.Create(&d)
	s := NewService(db, &config.Config{MediaRoot: root}, nil)
	if s.CompleteSeasonPack(context.Background(), d.ID, map[int]string{1: "unused-candidate.mkv"}, root) {
		t.Fatal("a wholly covered pack cannot replace the winner")
	}
	db.First(&d, d.ID)
	if d.Status != model.DownloadStatusSuperseded {
		t.Fatalf("covered pack kept retrying: %s", d.Status)
	}
	db.Model(&d).Update("status", model.DownloadStatusFailed)
	if s.CompleteSeasonPack(context.Background(), d.ID, map[int]string{2: "unused-candidate.mkv"}, root) {
		t.Fatal("failed pack resurrected")
	}
}

func TestPackIgnoresNumberedExtrasInBothModes(t *testing.T) {
	root := t.TempDir()
	for ep := 1; ep <= 4; ep++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("Nova S01E%02d.mkv", ep)), []byte("episode"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"OVA 05.mkv", "[SP05].mkv", "NCOP 05.mkv", "sample S01E05.mkv", "Extras/Series S01E05.mkv"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("extra"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := mapSeasonPackMediaFilesMode(root, 1, 5, false)
	if err != nil || len(files) != 4 || files[5] != "" {
		t.Fatalf("backfill: %v %v", files, err)
	}
	if _, err := mapSeasonPackMediaFilesMode(root, 1, 5, true); err == nil {
		t.Fatal("upgrade cannot use an extra for EP5")
	}
}

func TestBackfillNeverUsesOVAAsFinale(t *testing.T) {
	root := t.TempDir()
	for ep := 1; ep <= 11; ep++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("Series S01E%02d.mkv", ep)), []byte("episode"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	extra := filepath.Join(root, "OVA.mkv")
	if err := os.WriteFile(extra, []byte("bonus"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := mapSeasonPackMediaFilesMode(root, 1, 12, false)
	if err != nil || len(files) != 11 || files[12] != "" {
		t.Fatalf("missing EP12 must remain missing: %v %v", files, err)
	}
}
