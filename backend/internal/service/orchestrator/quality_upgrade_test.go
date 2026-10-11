package orchestrator

import (
	"context"
	"fmt"
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	dl "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompleteSeasonQualityUpgradeDecision(t *testing.T) {
	for _, tc := range []struct {
		name, subtitle, status string
		future, promote        bool
		want                   bool
	}{
		{name: "complete but traditional", subtitle: "繁体", status: model.AnimeStatusFinished, want: true},
		{name: "no repeated upgrade after goal", subtitle: "简体", status: model.AnimeStatusFinished},
		{name: "unknown hard subtitles", status: model.AnimeStatusFinished},
		{name: "ongoing records debt", subtitle: "繁体", status: model.AnimeStatusOngoing},
		{name: "future final overrides status", subtitle: "繁体", status: model.AnimeStatusFinished, future: true},
		{name: "promote existing pack", subtitle: "繁体", status: model.AnimeStatusFinished, promote: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.InitTestDB()
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(1)
			root := t.TempDir()
			total := 12
			a := model.Anime{Title: "Quality Test", EpisodeCount: &total, Status: tc.status, IsSubscribed: true, MediaManagementState: MediaStateTracking}
			if err := db.Create(&a).Error; err != nil {
				t.Fatal(err)
			}
			old := model.Download{TorrentID: "old", Name: "old", URL: "old", AnimeID: &a.ID, SourceTitle: "[Test] Quality Test [01-12][1080p][" + tc.subtitle + "]", Status: model.DownloadStatusCompleted, Scope: model.DownloadScopeSeason}
			db.Create(&old)
			dir := dl.BuildAnimeSavePath(root, &a)
			os.MkdirAll(dir, 0755)
			for ep := 1; ep <= 12; ep++ {
				p := filepath.Join(dir, fmt.Sprintf("Quality Test S01E%02d.mkv", ep))
				os.WriteFile(p, []byte("playable old file"), 0600)
				row := model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: ep, Downloaded: true, DownloadID: &old.TorrentID, FilePath: &p}
				if tc.future && ep == 12 {
					date := time.Now().AddDate(0, 0, 5).Format("2006-01-02")
					row.AirDate = &date
				}
				db.Create(&row)
			}
			items := []indexer.Candidate{}
			// These must never become the upgrade despite having more seeders.
			titles := []string{"[Test] Quality Test [01-06 Fin][1080p][简体]", "[Test] Quality Test [01-12 Fin][1080p][繁体]", "[Test] Quality Test [01-12 Fin][720p][简体]", "[Test] Quality Test Season 2 [01-12 Fin][1080p][简体]", "[Test] Quality Test [01-12 Fin][1080p][简体]"}
			for i, title := range titles {
				hash := strings.Repeat(fmt.Sprintf("%x", i+1), 40)
				items = append(items, indexer.Candidate{Title: title, InfoHash: hash, MagnetURL: "magnet:?xt=urn:btih:" + hash, SourceName: SourceMikan, Seeders: 100 - i})
			}
			svc := dl.NewService(db, &config.Config{MediaRoot: root}, nil)
			release := make(chan struct{})
			defer close(release)
			svc.RegisterExecutor(model.DownloadTypeTorrent, &raceQueueExecutor{release: release})
			o := New(db, svc, nil, nil, map[string]indexer.Indexer{SourceMikan: &raceQueueIndexer{items: items}}, root)
			calls := 0
			o.mediaProbe = func(context.Context, string) (mediacheck.Media, error) {
				calls++
				return mediacheck.Media{Streams: []mediacheck.Stream{{CodecType: "video", Width: 1920, Height: 1080}}}, nil
			}
			pref := Preference{Quality: "1080p", Languages: []string{"simplified"}, BTEnabled: true, EnabledIndexers: []string{SourceMikan}, Priority: []string{SourceMikan}}
			ctx := context.Background()
			q, err := o.auditLibraryQuality(ctx, &a, pref)
			if err != nil {
				t.Fatal(err)
			}
			if q.Available != 12 {
				t.Fatalf("available %d", q.Available)
			}
			if tc.subtitle == "繁体" && len(q.Needed) != 12 {
				t.Fatalf("debt not recorded %+v", q)
			}
			if tc.subtitle == "" && q.Unknown != 12 {
				t.Fatalf("unknown evidence must remain unknown %+v", q)
			}
			q2, err := o.auditLibraryQuality(ctx, &a, pref)
			if err != nil || calls != 12 {
				t.Fatalf("unchanged files probed again: %d %v", calls, err)
			}
			if tc.promote {
				start, end, ep := 1, 12, 1
				p := dl.BuildTorrentPackSavePath(root, a.ID, start, end, "pack", "magnet")
				d := model.Download{TorrentID: "existing", Name: "pack", URL: "magnet-existing", AnimeID: &a.ID, SourceTitle: titles[4], Status: model.DownloadStatusPending, Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end, EpisodeNumber: &ep, SavePath: &p}
				db.Create(&d)
			}
			if got := o.trySeasonUpgrade(ctx, &a, q2, pref); got != tc.want {
				t.Fatalf("upgrade=%v want %v %+v", got, tc.want, q2)
			}
			if tc.want {
				if !o.trySeasonUpgrade(ctx, &a, q2, pref) {
					t.Fatal("existing upgrade not retained")
				}
				var tasks []model.Download
				db.Where("purpose = ?", model.DownloadPurposeSeasonUpgrade).Find(&tasks)
				if len(tasks) != 1 {
					t.Fatalf("duplicate upgrades: %d", len(tasks))
				}
				if tasks[0].SourceTitle != titles[4] || !strings.Contains(tasks[0].UpgradeReason, "补齐 0 集，升级 12 集") {
					t.Fatalf("wrong upgrade %+v", tasks[0])
				}
				if tc.promote && tasks[0].TorrentID != "existing" {
					t.Fatal("pack redownloaded instead of promoted")
				}
			}
			var count int64
			db.Model(&model.AnimeEpisode{}).Where("anime_id = ? AND downloaded = ? AND download_id = ?", a.ID, true, "old").Count(&count)
			if count != 12 {
				t.Fatalf("old playback mappings changed: %d", count)
			}
		})
	}
}
