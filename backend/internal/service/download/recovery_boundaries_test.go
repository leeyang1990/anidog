package download

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestMissingCompletionAllowsReplacementAndPreservesHistory(t *testing.T) {
	db := testutil.InitTestDB()
	anime := model.Anime{Title: "missing winner"}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	ep := 9
	old := model.Download{TorrentID: "old", Name: "old", URL: "old", AnimeID: &anime.ID, EpisodeNumber: &ep,
		Status: model.DownloadStatusCompleted, MediaMissing: true}
	failed := model.Download{TorrentID: "failed", Name: "failed", URL: "failed", AnimeID: &anime.ID, EpisodeNumber: &ep,
		Status: model.DownloadStatusFailed, FailureKind: model.FailureKindRejected}
	replacement := model.Download{TorrentID: "new", Name: "new", URL: "new", AnimeID: &anime.ID, EpisodeNumber: &ep,
		Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeStream, MediaMissing: true}
	for _, row := range []*model.Download{&old, &failed, &replacement} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{db: db}
	// Startup cleanup and per-completion cleanup must not hide an unresolved failure.
	s.resolveHistoricalFailures(context.Background())
	s.resolvePriorFailures(old.ID)
	if err := db.First(&failed, failed.ID).Error; err != nil {
		t.Fatal(err)
	}
	if failed.Status != model.DownloadStatusFailed {
		t.Fatal("missing media falsely resolved failure")
	}
	dir := t.TempDir()
	staging, final := filepath.Join(dir, "new.part.mp4"), filepath.Join(dir, "final.mp4")
	if err := os.WriteFile(staging, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if !s.CompleteEpisodeRace(context.Background(), replacement.ID, staging, final) {
		t.Fatal("replacement must win")
	}
	if err := db.First(&replacement, replacement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if replacement.Status != model.DownloadStatusCompleted || replacement.MediaMissing || replacement.MediaMissingAt != nil {
		t.Fatalf("invalid replacement state: %+v", replacement)
	}
	if err := db.First(&old, old.ID).Error; err != nil {
		t.Fatal(err)
	}
	if old.Status != model.DownloadStatusCompleted || !old.MediaMissing {
		t.Fatal("historical loss must remain auditable")
	}
	if err := db.First(&failed, failed.ID).Error; err != nil {
		t.Fatal(err)
	}
	if failed.Status != model.DownloadStatusSuperseded {
		t.Fatal("real replacement must resolve failures")
	}
	var episode model.AnimeEpisode
	if err := db.Where("anime_id = ? AND episode_number = ?", anime.ID, ep).First(&episode).Error; err != nil {
		t.Fatal(err)
	}
	if !episode.Downloaded || episode.FilePath == nil || *episode.FilePath != final {
		t.Fatal("episode not repaired")
	}
	// Repeated completion must preserve the final file and must not elect/notify twice.
	if s.CompleteEpisodeRace(context.Background(), replacement.ID, final, final) {
		t.Fatal("duplicate completion")
	}
	if content, err := os.ReadFile(final); err != nil || string(content) != "replacement" {
		t.Fatalf("lost final: %s %v", content, err)
	}
}

func TestConcurrentEpisodeCompletionsElectOneSQLiteWinner(t *testing.T) {
	db := testutil.InitTestDB()
	anime := model.Anime{Title: "concurrent race"}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	ep := 1
	dir, start := t.TempDir(), make(chan struct{})
	final := filepath.Join(dir, "episode.mp4")
	var rows []model.Download
	for _, name := range []string{"first", "second"} {
		row := model.Download{TorrentID: name, Name: name, URL: name, AnimeID: &anime.ID, EpisodeNumber: &ep,
			Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeStream}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".part"), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	s := &Service{db: db}
	var wg sync.WaitGroup
	results := make(chan bool, len(rows))
	for _, row := range rows {
		wg.Go(func() {
			<-start
			results <- s.CompleteEpisodeRace(context.Background(), row.ID, filepath.Join(dir, row.Name+".part"), final)
		})
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for won := range results {
		if won {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("got %d winners", winners)
	}
	var winner model.Download
	if err := db.Scopes(AvailableMedia).First(&winner).Error; err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(final); err != nil || string(content) != winner.Name {
		t.Fatalf("winner file mismatch: %s %v", content, err)
	}
}

func TestLateCallbacksCannotResurrectInactiveDownloads(t *testing.T) {
	for _, status := range []string{model.DownloadStatusPaused, model.DownloadStatusFailed, model.DownloadStatusSuperseded, model.DownloadStatusCompleted} {
		t.Run(status, func(t *testing.T) {
			db := testutil.InitTestDB()
			row := model.Download{TorrentID: "late", Name: "late", URL: "late", Status: status}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			s := &Service{db: db}
			for _, update := range []string{model.DownloadStatusFailed, model.DownloadStatusDownloading} {
				if s.updateStatus(row.ID, update, nil) {
					t.Fatal("late callback changed inactive task")
				}
			}
			path := filepath.Join(t.TempDir(), "candidate.part")
			if err := os.WriteFile(path, []byte("candidate"), 0600); err != nil {
				t.Fatal(err)
			}
			if s.CompleteEpisodeRace(context.Background(), row.ID, path, path+".final") {
				t.Fatal("inactive task won")
			}
			if err := db.First(&row, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if row.Status != status {
				t.Fatal("task resurrected")
			}
			if status != model.DownloadStatusSuperseded {
				if _, err := os.Stat(path); err != nil {
					t.Fatal("paused/failed/completed callback discarded file")
				}
			}
		})
	}
}

func TestCompletionPreservesFilesWhenCanceledOrConflicting(t *testing.T) {
	for _, scenario := range []string{"canceled", "conflicting final"} {
		t.Run(scenario, func(t *testing.T) {
			db := testutil.InitTestDB()
			row := model.Download{TorrentID: "candidate", Name: "candidate", URL: "candidate", Status: model.DownloadStatusDownloading}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			s := &Service{db: db}
			dir := t.TempDir()
			staging, final := filepath.Join(dir, "candidate.part"), filepath.Join(dir, "final")
			if err := os.WriteFile(staging, []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			} else if err := os.WriteFile(final, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			if s.CompleteEpisodeRace(ctx, row.ID, staging, final) {
				t.Fatal("must not complete")
			}
			if got, err := os.ReadFile(staging); err != nil || string(got) != "new" {
				t.Fatal("candidate lost")
			}
			if scenario == "conflicting final" {
				if got, err := os.ReadFile(final); err != nil || string(got) != "existing" {
					t.Fatal("existing media overwritten")
				}
			}
		})
	}
}
