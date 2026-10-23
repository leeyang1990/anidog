package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	dl "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/service/setting"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestCompletionAuditUpdatesQualityWithoutCreatingDownloads(t *testing.T) {
	db := testutil.InitTestDB()
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	root := t.TempDir()
	count := 1
	a := model.Anime{Title: "completion-audit", EpisodeCount: &count, IsSubscribed: true, Status: model.AnimeStatusFinished}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dl.BuildAnimeSavePath(root, &a), "episode.mp4")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture media"), 0600); err != nil {
		t.Fatal(err)
	}
	d := model.Download{TorrentID: "verified", Name: "verified", URL: "play", Status: model.DownloadStatusCompleted, VerifiedLanguages: `["simplified"]`, AnimeID: &a.ID}
	if err := db.Create(&d).Error; err != nil {
		t.Fatal(err)
	}
	row := model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: 1, Downloaded: true, DownloadID: &d.TorrentID, FilePath: &path}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	svc := setting.NewService(nil, db)
	o := New(db, nil, nil, svc, nil, root)
	o.mediaProbe = func(context.Context, string) (mediacheck.Media, error) {
		return mediacheck.Media{Streams: []mediacheck.Stream{{CodecType: "video", Width: 1920, Height: 1080}}}, nil
	}
	if err := o.RefreshLibraryQuality(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.QualityStatus != model.QualityMeetsTarget || row.QualityFingerprint == "" {
		t.Fatal("completed proof was not projected", row)
	}
	if err := svc.Set(context.Background(), "download.languages", `["traditional"]`); err != nil {
		t.Fatal(err)
	}
	if err := o.RefreshLibraryQuality(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.QualityStatus != model.QualityUpgradeNeeded || !row.Downloaded {
		t.Fatal("policy change reused old proof or removed playback", row)
	}
	var tasks int64
	db.Model(&model.Download{}).Count(&tasks)
	if tasks != 1 {
		t.Fatal("quality event created replacement tasks", tasks)
	}
}
