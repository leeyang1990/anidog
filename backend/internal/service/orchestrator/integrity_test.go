package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	dl "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestArchivedSeasonWithNeverDownloadedFinaleReopens(t *testing.T) {
	db := testutil.InitTestDB()
	root := t.TempDir()
	total := 2
	a := model.Anime{Title: "Finale", IsSubscribed: true, EpisodeCount: &total, MediaManagementState: MediaStateArchived}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	dir := dl.BuildAnimeSavePath(root, &a)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Finale S01E01.mkv")
	if err := os.WriteFile(path, []byte("existing episode"), 0600); err != nil {
		t.Fatal(err)
	}
	date := "2020-01-01"
	for ep := 1; ep <= total; ep++ {
		r := model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: ep, AirDate: &date, Downloaded: ep == 1}
		if ep == 1 {
			r.FilePath = &path
		}
		if err := db.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
	}
	o := New(db, nil, nil, nil, map[string]indexer.Indexer{}, root)
	o.CheckAnime(context.Background(), &a, Preference{})
	if a.MediaManagementState != MediaStateFinalizing {
		t.Fatalf("missing finale stayed %s", a.MediaManagementState)
	}
	second := filepath.Join(dir, "Finale S01E02.mkv")
	if err := os.WriteFile(second, []byte("second episode"), 0600); err != nil {
		t.Fatal(err)
	}
	o.CheckAnime(context.Background(), &a, Preference{})
	if a.MediaManagementState != MediaStateArchived {
		t.Fatalf("complete season stayed %s", a.MediaManagementState)
	}
}

func TestArchiveRequiresAllActualEpisodes(t *testing.T) {
	db := testutil.InitTestDB()
	total := 3
	a := model.Anime{Title: "audit", EpisodeCount: &total, MediaManagementState: MediaStateFinalizing}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{db: db}
	for _, tc := range []struct {
		present map[int]bool
		want    string
	}{
		{map[int]bool{1: true, 2: true}, MediaStateFinalizing},
		{map[int]bool{1: true, 2: true, 3: true}, MediaStateArchived},
		{map[int]bool{1: true, 3: true}, MediaStateFinalizing},
	} {
		o.persistMediaAuditResult(context.Background(), &a, a.MediaManagementState, 3, 3, MediaAuditResult{Success: true, Confirmed: true, Present: tc.present})
		if a.MediaManagementState != tc.want {
			t.Fatalf("present=%v state=%s want %s", tc.present, a.MediaManagementState, tc.want)
		}
	}
}

func TestCompletedPackCannotInventEpisodeOne(t *testing.T) {
	db := testutil.InitTestDB()
	a := model.Anime{Title: "partial pack"}
	db.Create(&a)
	ep, start, end := 1, 1, 12
	db.Create(&model.Download{TorrentID: "pack", Name: "pack", URL: "pack", AnimeID: &a.ID, EpisodeNumber: &ep, Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end, Status: model.DownloadStatusCompleted})
	db.Create(&model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: 2, Downloaded: true, UpdatedAt: time.Now()})
	o := &Orchestrator{db: db}
	covered := o.downloadedEpisodes(context.Background(), a.ID)
	if covered[1] || !covered[2] || covered[12] {
		t.Fatalf("invented coverage: %v", covered)
	}
}

func TestRejectedStreamStagingIsNeverAdoptedAsLibraryMedia(t *testing.T) {
	db := testutil.InitTestDB()
	root := t.TempDir()
	total := 1
	a := model.Anime{Title: "Staging", IsSubscribed: true, EpisodeCount: &total, MediaManagementState: MediaStateFinalizing}
	db.Create(&a)
	dir := dl.BuildAnimeSavePath(root, &a)
	os.MkdirAll(dir, 0755)
	staging := filepath.Join(dir, "Staging S01E01.anidog-stream_123.part.mp4")
	os.WriteFile(staging, []byte("rejected video"), 0600)
	date := "2020-01-01"
	e := model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: 1, AirDate: &date}
	db.Create(&e)
	ep := 1
	db.Create(&model.Download{TorrentID: "stream_123", Name: "candidate", URL: "candidate", AnimeID: &a.ID, EpisodeNumber: &ep, Status: model.DownloadStatusFailed, FailureKind: model.FailureKindRejected, FilePath: &staging})
	o := New(db, nil, nil, nil, map[string]indexer.Indexer{}, root)
	o.CheckAnime(context.Background(), &a, Preference{})
	db.First(&e, e.ID)
	if e.Downloaded || a.MediaManagementState == MediaStateArchived {
		t.Fatal("rejected staging bypassed the completion guard through disk adoption")
	}
	final := filepath.Join(dir, "Staging S01E01.mp4")
	os.WriteFile(final, []byte("actual archive"), 0600)
	o.CheckAnime(context.Background(), &a, Preference{})
	db.First(&e, e.ID)
	if !e.Downloaded || e.FilePath == nil || *e.FilePath != final || e.FileSize == nil || *e.FileSize != int64(len("actual archive")) {
		t.Fatalf("disk adoption did not preserve actual file evidence: %+v", e)
	}
}
