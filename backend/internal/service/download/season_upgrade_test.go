package download

import (
	"context"
	"fmt"
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"github.com/anidog/anidog-go/internal/testutil"
	"gorm.io/gorm"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func upgradeTestVideo(t *testing.T) []byte {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	p := filepath.Join(t.TempDir(), "fixture.mkv")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=1920x1080:r=2", "-t", "3", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestSeasonUpgradeAllOrNothing(t *testing.T) {
	video := upgradeTestVideo(t)
	for _, tc := range []struct {
		name                                     string
		broken, missing, dbFailure, pauseFailure bool
		ext                                      string
	}{
		{name: "same extension", ext: ".mkv"}, {name: "different extension", ext: ".mp4"},
		{name: "one bad episode", ext: ".mp4", broken: true}, {name: "incomplete pack", ext: ".mkv", missing: true},
		{name: "database failure after moves", ext: ".mkv", dbFailure: true},
		{name: "cannot stop file access", ext: ".mkv", pauseFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.InitTestDB()
			root := t.TempDir()
			total := 2
			a := model.Anime{Title: "Upgrade", EpisodeCount: &total, Status: model.AnimeStatusFinished}
			db.Create(&a)
			dir := BuildAnimeSavePath(root, &a)
			os.MkdirAll(filepath.Join(dir, "Extras"), 0755)
			os.WriteFile(filepath.Join(dir, "Extras", "bonus.mkv"), []byte("bonus"), 0600)
			os.WriteFile(filepath.Join(dir, "season.nfo"), []byte("metadata"), 0600)
			oldPaths := map[int]string{}
			for ep := 1; ep <= total; ep++ {
				p := filepath.Join(dir, fmt.Sprintf("Upgrade S01E%02d%s", ep, tc.ext))
				oldPaths[ep] = p
				os.WriteFile(p, []byte(fmt.Sprintf("old-%d", ep)), 0600)
				id := fmt.Sprintf("old-%d", ep)
				db.Create(&model.Download{TorrentID: id, Name: id, URL: id, AnimeID: &a.ID, EpisodeNumber: &ep, Status: model.DownloadStatusCompleted, FilePath: &p})
				db.Create(&model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: ep, Downloaded: true, DownloadID: &id, FilePath: &p, QualityStatus: model.QualityUpgradeNeeded})
			}
			os.WriteFile(filepath.Join(dir, "Upgrade S01E01.cht.ass"), []byte("old subs"), 0600)
			start, end, trigger := 1, 2, 1
			staging := BuildTorrentPackSavePath(root, a.ID, start, end, "PACK", "magnet")
			os.MkdirAll(staging, 0755)
			files := map[int]string{}
			for ep := 1; ep <= total; ep++ {
				p := filepath.Join(staging, fmt.Sprintf("episode-%02d.mkv", ep))
				os.WriteFile(p, video, 0600)
				files[ep] = p
			}
			os.WriteFile(filepath.Join(staging, "episode-01.chs.ass"), []byte("new subs"), 0600)
			if tc.broken {
				os.WriteFile(files[2], []byte("corrupt"), 0600)
			}
			if tc.missing {
				delete(files, 2)
			}
			candidate := model.Download{TorrentID: "upgrade", Name: "upgrade", URL: "magnet", AnimeID: &a.ID, EpisodeNumber: &trigger, EpisodeStart: &start, EpisodeEnd: &end, Scope: model.DownloadScopeSeason, Purpose: model.DownloadPurposeSeasonUpgrade, DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusDownloading, SavePath: &staging, SourceTitle: "[Test] Upgrade [01-02][1080p][简体]", UpgradeTarget: QualityGoalKey("1080p", []string{"simplified"})}
			db.Create(&candidate)
			// Even with ordinary verification disabled, an upgrade must validate each file.
			svc := NewService(db, &config.Config{MediaRoot: root, VerifyDownloadedMedia: false}, nil)
			gate := &upgradeFileGate{pause: func() error {
				for _, path := range files {
					if _, err := os.Stat(path); err != nil {
						t.Fatal("candidate moved before file access stopped", err)
					}
				}
				for _, path := range oldPaths {
					if _, err := os.Stat(path); err != nil {
						t.Fatal("old season moved before file access stopped", err)
					}
				}
				if tc.pauseFailure {
					return fmt.Errorf("injected stop failure")
				}
				return nil
			}}
			svc.RegisterExecutor(model.DownloadTypeTorrent, gate)

			if tc.dbFailure {
				db.Callback().Update().Before("gorm:update").Register("test:fail-upgrade", func(tx *gorm.DB) {
					if tx.Statement.Table == "anime" {
						tx.AddError(fmt.Errorf("injected database failure"))
					}
				})
				defer db.Callback().Update().Remove("test:fail-upgrade")
			}
			ok := svc.CompleteSeasonPack(context.Background(), candidate.ID, files, root)
			want := !tc.broken && !tc.missing && !tc.dbFailure && !tc.pauseFailure
			if ok != want {
				var d model.Download
				db.First(&d, candidate.ID)
				t.Fatalf("completed=%v want %v error=%v", ok, want, d.LastError)
			}
			db.First(&candidate, candidate.ID)
			wantCalls := 1
			if tc.broken || tc.missing {
				wantCalls = 0
			}
			if gate.calls != wantCalls {
				t.Fatalf("file gate calls %d want %d", gate.calls, wantCalls)
			}
			if tc.dbFailure || tc.pauseFailure {
				if candidate.Status != model.DownloadStatusPaused {
					t.Fatalf("failed swap must pause, got %s", candidate.Status)
				}
				before, _ := filepath.Glob(filepath.Join(root, ".anidog-upgrades", "anime-*", "task-*", "*"))
				if svc.CompleteSeasonPack(context.Background(), candidate.ID, files, root) {
					t.Fatal("paused failed swap ran again")
				}
				after, _ := filepath.Glob(filepath.Join(root, ".anidog-upgrades", "anime-*", "task-*", "*"))
				if len(after) != len(before) {
					t.Fatal("failed swap created another attempt")
				}
			}
			for ep, p := range oldPaths {
				var row model.AnimeEpisode
				db.Where("anime_id = ? AND episode_number = ?", a.ID, ep).First(&row)
				if !want {
					data, _ := os.ReadFile(p)
					if string(data) != fmt.Sprintf("old-%d", ep) || row.DownloadID == nil || *row.DownloadID != fmt.Sprintf("old-%d", ep) {
						t.Fatalf("old episode %d lost: %+v", ep, row)
					}
					continue
				}
				if row.DownloadID == nil || *row.DownloadID != "upgrade" || row.QualityStatus != model.QualityMeetsTarget {
					t.Fatalf("mapping not replaced %+v", row)
				}
				if data, _ := os.ReadFile(*row.FilePath); len(data) != len(video) {
					t.Fatal("new video missing")
				}
				if data, _ := os.ReadFile(filepath.Join(candidate.BackupPath, filepath.Base(p))); string(data) != fmt.Sprintf("old-%d", ep) {
					t.Fatal("recoverable backup missing")
				}
			}
			if want {
				for _, p := range []string{filepath.Join(dir, "Extras", "bonus.mkv"), filepath.Join(dir, "season.nfo"), filepath.Join(dir, "Upgrade S01E01.chs.ass"), filepath.Join(candidate.BackupPath, "Upgrade S01E01.cht.ass")} {
					if _, err := os.Stat(p); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := os.Stat(filepath.Join(dir, "Upgrade S01E01.cht.ass")); !os.IsNotExist(err) {
					t.Fatal("old subtitle mixed into new season")
				}
				if svc.CompleteSeasonPack(context.Background(), candidate.ID, files, root) {
					t.Fatal("completed upgrade ran twice")
				}
			}
			if err := svc.RecoverSeasonUpgrades(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpgradeRecoveryAtEveryMoveBoundary(t *testing.T) {
	for prefix := 0; prefix <= 4; prefix++ {
		t.Run(fmt.Sprintf("after-%d-moves", prefix), func(t *testing.T) {
			db := testutil.InitTestDB()
			root := t.TempDir()
			old := filepath.Join(root, "season")
			attempt := filepath.Join(root, ".anidog-upgrades", "anime-1", "task-1", "attempt")
			ready := filepath.Join(attempt, "new-season")
			backup := filepath.Join(attempt, "previous-season")
			input := filepath.Join(root, "staging")
			for _, p := range []string{old, ready, input} {
				os.MkdirAll(p, 0755)
			}
			os.WriteFile(filepath.Join(old, "old.mkv"), []byte("old"), 0600)
			for i := 1; i <= 2; i++ {
				os.WriteFile(filepath.Join(input, fmt.Sprint(i)), []byte("new"), 0600)
			}
			j := upgradeJournal{DownloadID: 1, AnimeID: 1, State: "prepared", BackupPath: backup, Moves: []upgradeMove{{filepath.Join(input, "1"), filepath.Join(ready, "1")}, {filepath.Join(input, "2"), filepath.Join(ready, "2")}, {old, backup}, {ready, old}}}
			path := filepath.Join(attempt, "journal.json")
			if err := writeUpgradeJournal(path, &j); err != nil {
				t.Fatal(err)
			}
			for _, m := range j.Moves[:prefix] {
				if err := os.Rename(m.From, m.To); err != nil {
					t.Fatal(err)
				}
			}
			svc := NewService(db, &config.Config{MediaRoot: root}, nil)
			for n := 0; n < 2; n++ {
				if err := svc.RecoverSeasonUpgrades(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if data, _ := os.ReadFile(filepath.Join(old, "old.mkv")); string(data) != "old" {
				t.Fatal("old library not restored")
			}
			for i := 1; i <= 2; i++ {
				if data, _ := os.ReadFile(filepath.Join(input, fmt.Sprint(i))); string(data) != "new" {
					t.Fatal("incoming file lost during recovery")
				}
			}
		})
	}
}

func TestTerminalUpgradeJournalLeavesStartupScan(t *testing.T) {
	for _, state := range []string{"committed", "rolled_back"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".anidog-upgrades", "anime-1", "task-1", "attempt", "journal.json")
			j := upgradeJournal{DownloadID: 1, State: state}
			if err := writeUpgradeJournal(path, &j); err != nil {
				t.Fatal(err)
			}
			archived := filepath.Join(filepath.Dir(path), "journal."+state+".json")
			if _, err := os.Stat(archived); err != nil {
				t.Fatal("terminal journal was not preserved", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("terminal journal remains in startup scan")
			}
			// Old releases left terminal records under the active filename.
			if err := os.Rename(archived, path); err != nil {
				t.Fatal(err)
			}
			svc := NewService(testutil.InitTestDB(), &config.Config{MediaRoot: root}, nil)
			if err := svc.RecoverSeasonUpgrades(context.Background()); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(archived); err != nil {
				t.Fatal("legacy terminal journal was not archived", err)
			}
		})
	}
}

func TestStrictUpgradePackDoesNotFillFinaleWithExtras(t *testing.T) {
	for _, extra := range []string{"[OVA].mkv", "SP/03.mkv", "Extras/03.mkv", "sample-03.mkv"} {
		t.Run(extra, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"episode-01.mkv", "episode-02.mkv", extra} {
				p := filepath.Join(dir, name)
				os.MkdirAll(filepath.Dir(p), 0755)
				os.WriteFile(p, []byte("x"), 0600)
			}
			if _, err := mapSeasonPackMediaFilesMode(dir, 1, 3, true); err == nil {
				t.Fatal("extra substituted for missing finale")
			}
		})
	}
	dir := t.TempDir()
	for _, name := range []string{"episode-01.mkv", "episode-01.mp4"} {
		os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600)
	}
	if _, err := mapSeasonPackMediaFilesMode(dir, 1, 1, true); err == nil {
		t.Fatal("duplicate mapping accepted")
	}
}
func TestActualSubtitleEvidenceOverridesPackTitle(t *testing.T) {
	m := mediacheck.Media{Streams: []mediacheck.Stream{{CodecType: "video", Width: 1920, Height: 1080}, {CodecType: "subtitle", Tags: map[string]string{"title": "CHT", "language": "chi"}}}}
	if got := AssessMediaPreferences(m, "missing.mkv", "[Test] Pack [简繁内封]", "1080p", []string{"simplified"}); got.Status != model.QualityUpgradeNeeded {
		t.Fatalf("misleading title accepted: %+v", got)
	}
}

func TestUpgradeRecoveryKeepsCommittedDatabaseWinner(t *testing.T) {
	db := testutil.InitTestDB()
	root := t.TempDir()
	attempt := filepath.Join(root, ".anidog-upgrades", "anime-1", "task-1", "attempt")
	backup := filepath.Join(attempt, "previous-season")
	old := filepath.Join(root, "season")
	ready := filepath.Join(attempt, "new-season")
	os.MkdirAll(old, 0755)
	os.MkdirAll(backup, 0755)
	os.WriteFile(filepath.Join(old, "new.mkv"), []byte("new"), 0600)
	os.WriteFile(filepath.Join(backup, "old.mkv"), []byte("old"), 0600)
	d := model.Download{TorrentID: "upgrade", Name: "upgrade", URL: "magnet", Status: model.DownloadStatusCompleted, BackupPath: backup}
	db.Create(&d)
	j := upgradeJournal{DownloadID: d.ID, AnimeID: 1, State: "prepared", BackupPath: backup, Moves: []upgradeMove{{old, backup}, {ready, old}}}
	if err := writeUpgradeJournal(filepath.Join(attempt, "journal.json"), &j); err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, &config.Config{MediaRoot: root}, nil)
	if err := svc.RecoverSeasonUpgrades(context.Background()); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(old, "new.mkv")); string(data) != "new" {
		t.Fatal("committed new season rolled back")
	}
	if data, _ := os.ReadFile(filepath.Join(backup, "old.mkv")); string(data) != "old" {
		t.Fatal("old backup removed")
	}
}
func TestSingleEpisodeWinnerDoesNotCancelWholeSeasonUpgrade(t *testing.T) {
	db := testutil.InitTestDB()
	a := model.Anime{Title: "Upgrade"}
	db.Create(&a)
	ep := 1
	winner := model.Download{TorrentID: "single", Name: "single", URL: "single", AnimeID: &a.ID, EpisodeNumber: &ep, Status: model.DownloadStatusCompleted}
	db.Create(&winner)
	upgrade := model.Download{TorrentID: "pack", Name: "pack", URL: "pack", AnimeID: &a.ID, EpisodeNumber: &ep, Status: model.DownloadStatusDownloading, Purpose: model.DownloadPurposeSeasonUpgrade}
	db.Create(&upgrade)
	svc := NewService(db, &config.Config{}, nil)
	svc.settleEpisodeRace(context.Background(), &winner)
	svc.settleSeasonPack(context.Background(), &winner, []int{1, 2})
	db.First(&upgrade, upgrade.ID)
	if upgrade.Status != model.DownloadStatusDownloading {
		t.Fatal("quality upgrade canceled by ordinary backfill")
	}
}

type upgradeFileGate struct {
	pause func() error
	calls int
}

func (g *upgradeFileGate) Execute(context.Context, *Task, ProgressCallback) (*Result, error) {
	return &Result{}, nil
}
func (g *upgradeFileGate) Pause(string) error        { g.calls++; return g.pause() }
func (g *upgradeFileGate) Resume(string) error       { return nil }
func (g *upgradeFileGate) Cancel(string) error       { return nil }
func (g *upgradeFileGate) Remove(string, bool) error { return nil }
