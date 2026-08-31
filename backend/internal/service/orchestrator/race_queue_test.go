package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	downloadservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/service/titleparse"
	"github.com/anidog/anidog-go/internal/testutil"
)

type raceQueueIndexer struct {
	items []indexer.Candidate
}

func (i *raceQueueIndexer) Name() string { return SourceMikan }
func (i *raceQueueIndexer) Search(context.Context, string) ([]indexer.Candidate, error) {
	return append([]indexer.Candidate(nil), i.items...), nil
}

type raceQueueExecutor struct {
	release <-chan struct{}
}

func (e *raceQueueExecutor) Execute(_ context.Context, task *downloadservice.Task, _ downloadservice.ProgressCallback) (*downloadservice.Result, error) {
	<-e.release
	return &downloadservice.Result{TorrentID: downloadservice.ExtractInfoHash(task.URL)}, nil
}
func (*raceQueueExecutor) Cancel(string) error       { return nil }
func (*raceQueueExecutor) Pause(string) error        { return nil }
func (*raceQueueExecutor) Resume(string) error       { return nil }
func (*raceQueueExecutor) Remove(string, bool) error { return nil }

func TestSingleAvailableTierQueuesOnlyHighestPriorityCandidate(t *testing.T) {
	db := testutil.InitTestDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Service.Create starts executors asynchronously. A single SQLite writer
	// keeps this test focused on queue selection instead of SQLite lock timing.
	sqlDB.SetMaxOpenConns(1)
	anime := model.Anime{Title: "Queue Test", IsSubscribed: true}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}

	hashes := []string{
		strings.Repeat("G", 40),
		strings.Repeat("H", 40),
		strings.Repeat("I", 40),
	}
	items := make([]indexer.Candidate, 0, len(hashes))
	for n, hash := range hashes {
		items = append(items, indexer.Candidate{
			Title:      "[Test] Queue Test - 02 [1080p]",
			MagnetURL:  "magnet:?xt=urn:btih:" + hash,
			InfoHash:   hash,
			SourceName: "mikan",
			Seeders:    len(hashes) - n,
		})
	}

	dlSvc := downloadservice.NewService(db, &config.Config{}, nil)
	release := make(chan struct{})
	dlSvc.RegisterExecutor(model.DownloadTypeTorrent, &raceQueueExecutor{release: release})
	orch := New(db, dlSvc, nil, nil,
		map[string]indexer.Indexer{SourceMikan: &raceQueueIndexer{items: items}},
		t.TempDir())
	pref := Preference{
		BTEnabled:       true,
		EnabledIndexers: []string{SourceMikan},
		Priority:        []string{SourceMikan},
	}

	if !orch.tryDownloadEpisode(context.Background(), &anime, 2, pref) {
		t.Fatal("expected at least one candidate to be queued")
	}
	close(release)

	deadline := time.Now().Add(time.Second)
	var rows []model.Download
	for time.Now().Before(deadline) {
		rows = nil
		db.Where("anime_id = ? AND episode_number = ?", anime.ID, 2).Find(&rows)
		if len(rows) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(rows) != 1 {
		var diag []model.OrchestratorDiagnosis
		db.Where("anime_id = ? AND episode_number = ?", anime.ID, 2).Find(&diag)
		t.Fatalf("queued %d candidates, want 1; rows=%+v diag=%+v",
			len(rows), rows, diag)
	}
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.InfoHash == nil || *row.InfoHash == "" {
			t.Fatalf("candidate %d has no info hash", row.ID)
		}
		seen[*row.InfoHash] = true
		if row.Source != SourceMikan {
			t.Fatalf("candidate source=%q, want %q", row.Source, SourceMikan)
		}
		if row.SavePath == nil || !downloadservice.IsTorrentRaceSavePath(*row.SavePath) {
			t.Fatalf("candidate is not isolated from media library: %v", row.SavePath)
		}
	}
	if len(seen) != 1 {
		t.Fatalf("queued hashes are not distinct: %v", seen)
	}
}

func TestShouldPreferSeasonPack(t *testing.T) {
	for _, tc := range []struct {
		missing, expected int
		want              bool
	}{
		{25, 25, true},
		{4, 12, true},
		{3, 12, false},
		{4, 24, false},
	} {
		if got := shouldPreferSeasonPack(tc.missing, tc.expected); got != tc.want {
			t.Fatalf("shouldPreferSeasonPack(%d, %d)=%v, want %v", tc.missing, tc.expected, got, tc.want)
		}
	}
}

func TestTrySeasonPackQueuesOnlyOneHighestCoveragePack(t *testing.T) {
	db := testutil.InitTestDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	episodeCount := 12
	anime := model.Anime{Title: "Pack Test", EpisodeCount: &episodeCount, IsSubscribed: true}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	items := []indexer.Candidate{
		{Title: "[Test] Pack Test [01-06 Fin][1080p][简体]", MagnetURL: "magnet:?xt=urn:btih:" + strings.Repeat("J", 40), InfoHash: strings.Repeat("J", 40), SourceName: SourceMikan},
		{Title: "[Test] Pack Test [01-12 Fin][1080p][简体]", MagnetURL: "magnet:?xt=urn:btih:" + strings.Repeat("K", 40), InfoHash: strings.Repeat("K", 40), SourceName: SourceMikan},
	}
	dlSvc := downloadservice.NewService(db, &config.Config{}, nil)
	release := make(chan struct{})
	dlSvc.RegisterExecutor(model.DownloadTypeTorrent, &raceQueueExecutor{release: release})
	orch := New(db, dlSvc, nil, nil,
		map[string]indexer.Indexer{SourceMikan: &raceQueueIndexer{items: items}}, t.TempDir())
	pref := Preference{BTEnabled: true, EnabledIndexers: []string{SourceMikan}, Priority: []string{SourceMikan}}
	missing := make([]int, 12)
	for i := range missing {
		missing[i] = i + 1
	}
	if !orch.trySeasonPack(context.Background(), &anime, missing, 12, pref) {
		t.Fatal("expected a season pack")
	}
	close(release)
	var rows []model.Download
	db.Where("anime_id = ?", anime.ID).Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("queued %d packs, want 1", len(rows))
	}
	if rows[0].Scope != model.DownloadScopeSeason || rows[0].EpisodeStart == nil || rows[0].EpisodeEnd == nil ||
		*rows[0].EpisodeStart != 1 || *rows[0].EpisodeEnd != 12 {
		t.Fatalf("unexpected pack row: %+v", rows[0])
	}
}

func TestRetainPreferredLanguageRejectsExplicitConflicts(t *testing.T) {
	candidates := []indexer.ScoredCandidate{
		{Candidate: indexer.Candidate{Title: "CHS", Parsed: titleparse.Parse("[Test] Show [02][简体内嵌]")}},
		{Candidate: indexer.Candidate{Title: "CHT", Parsed: titleparse.Parse("[Test] Show [02][繁体内嵌]")}},
		{Candidate: indexer.Candidate{Title: "BOTH", Parsed: titleparse.Parse("[Test] Show [02][简繁内封]")}},
	}
	got := retainPreferredLanguage(candidates, []string{"simplified"})
	if len(got) != 2 || got[0].Title != "CHS" || got[1].Title != "BOTH" {
		t.Fatalf("simplified preference kept unexpected candidates: %+v", got)
	}
	fallback := retainPreferredLanguage(candidates, []string{"japanese"})
	if len(fallback) != len(candidates) {
		t.Fatalf("unavailable preferred language must retain fallback candidates")
	}
}
