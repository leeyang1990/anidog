package download

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/testutil"
)

type pathCaptureExecutor struct{ paths chan string }

func (e *pathCaptureExecutor) Execute(_ context.Context, task *Task, _ ProgressCallback) (*Result, error) {
	e.paths <- task.SavePath
	return &Result{TorrentID: task.RuntimeID}, nil
}
func (*pathCaptureExecutor) Cancel(string) error       { return nil }
func (*pathCaptureExecutor) Pause(string) error        { return nil }
func (*pathCaptureExecutor) Resume(string) error       { return nil }
func (*pathCaptureExecutor) Remove(string, bool) error { return nil }

func TestResolvedPackDirectoryReachesExecutor(t *testing.T) {
	db := testutil.InitTestDB()
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	s := NewService(db, &config.Config{MediaRoot: t.TempDir()}, nil)
	e := &pathCaptureExecutor{paths: make(chan string, 1)}
	s.RegisterExecutor(model.DownloadTypeTorrent, e)
	a := model.Anime{Title: "Pack"}
	db.Create(&a)
	first, last := 1, 12
	row, err := s.Create(context.Background(), &Task{Name: "Pack", URL: "magnet:?xt=urn:btih:" + strings.Repeat("A", 40), DownloadType: model.DownloadTypeTorrent, Source: SourceBT, AnimeID: &a.ID, EpisodeNumber: &first, Scope: model.DownloadScopeSeason, EpisodeStart: &first, EpisodeEnd: &last})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case path := <-e.paths:
		if row.SavePath == nil || path != *row.SavePath || !IsTorrentRaceSavePath(path) {
			t.Fatalf("executor=%q DB=%v", path, row.SavePath)
		}
	case <-time.After(time.Second):
		t.Fatal("executor did not run")
	}
}

func TestFutureEpisodeGuardCoversRSSAndPacks(t *testing.T) {
	db := testutil.InitTestDB()
	a := model.Anime{Title: "Future"}
	db.Create(&a)
	first, last := 1, 2
	future := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	db.Create(&model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: 2, AirDate: &future})
	s := NewService(db, &config.Config{}, nil)
	for _, task := range []*Task{
		{Source: SourceRSS, AnimeID: &a.ID, EpisodeNumber: &last},
		{Source: SourceBT, AnimeID: &a.ID, EpisodeNumber: &first, Scope: model.DownloadScopeSeason, EpisodeStart: &first, EpisodeEnd: &last},
	} {
		if err := s.validateTaskAirDate(context.Background(), task); err == nil {
			t.Fatal("future content admitted")
		}
	}
	if err := s.validateTaskAirDate(context.Background(), &Task{Source: SourceManual, AnimeID: &a.ID, EpisodeNumber: &last}); err != nil {
		t.Fatal(err)
	}
}

func TestEpisodeSizeUsesVideoInsteadOfTorrentPayload(t *testing.T) {
	db := testutil.InitTestDB()
	a := model.Anime{Title: "size"}
	db.Create(&a)
	ep := 2
	size := int64(6_838_324_364)
	path := filepath.Join(t.TempDir(), "episode.mkv")
	if err := os.WriteFile(path, []byte("video bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewService(db, &config.Config{}, nil)
	if err := s.syncCompletedEpisodeState(context.Background(), &model.Download{AnimeID: &a.ID, EpisodeNumber: &ep, Status: model.DownloadStatusCompleted, TorrentID: "size", FilePath: &path, TotalBytes: &size}); err != nil {
		t.Fatal(err)
	}
	var row model.AnimeEpisode
	db.Where("anime_id = ?", a.ID).First(&row)
	if row.FileSize == nil || *row.FileSize != 11 {
		t.Fatalf("size=%v", row.FileSize)
	}
}

func TestReferenceRejectsPollutedShortBaseline(t *testing.T) {
	for _, actual := range []float64{536, 738, 1077, 827} {
		if ValidateReferenceDuration(actual, 1430) == nil {
			t.Errorf("accepted %v / 1430", actual)
		}
	}
	for _, tc := range [][2]float64{{1430, 1430}, {390, 400}, {1420, 1430}, {0, 0}} {
		if err := ValidateReferenceDuration(tc[0], int(tc[1])); err != nil {
			t.Error(err)
		}
	}
	if validateDurationAgainstPeers(827, []float64{1420, 1420, 1420}) == nil {
		t.Fatal("full-length peer guard accepted 13-minute capture")
	}
}

func TestActualMediaPreferenceEvidence(t *testing.T) {
	m := mediacheck.Media{Streams: []mediacheck.Stream{{CodecType: "video", Width: 1280, Height: 720}, {CodecType: "audio", Tags: map[string]string{"language": "eng"}}, {CodecType: "subtitle", Tags: map[string]string{"language": "eng"}}}}
	if validateMediaPreferences(m, "", "[组][简体]", "1080p", []string{"simplified"}) == nil {
		t.Fatal("advertised 1080p hid actual 720p")
	}
	m.Streams[0].Width = 1920
	m.Streams[0].Height = 1080
	if validateMediaPreferences(m, "", "CR release", "1080p", []string{"simplified"}) == nil {
		t.Fatal("English-only tracks satisfied Chinese subtitles")
	}
	m.Streams = append(m.Streams, mediacheck.Stream{CodecType: "subtitle", Tags: map[string]string{"language": "chi", "title": "中文（中国）"}})
	if err := validateMediaPreferences(m, "", "CR release", "1080p", []string{"simplified"}); err != nil {
		t.Fatal(err)
	}
}
