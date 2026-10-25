package download

import (
	"context"
	"database/sql"
	"errors"
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type commitAckPool struct {
	gorm.ConnPool
	inject   atomic.Bool
	failRead atomic.Bool
	mode     string
	cancel   context.CancelFunc
}

func (p *commitAckPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if p.failRead.Load() {
		return nil, errors.New("database unavailable after commit")
	}
	return p.ConnPool.QueryContext(ctx, query, args...)
}

func (p *commitAckPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	if p.failRead.Load() {
		return nil, errors.New("database unavailable after commit")
	}
	tx, err := p.ConnPool.(gorm.TxBeginner).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &commitAckTx{Tx: tx, pool: p}, nil
}

type commitAckTx struct {
	*sql.Tx
	pool *commitAckPool
}

func (t *commitAckTx) Commit() error {
	if t.pool.inject.Load() && t.pool.mode == "not committed" {
		_ = t.Tx.Rollback()
		t.pool.inject.Store(false)
		return errors.New("commit rejected")
	}
	if err := t.Tx.Commit(); err != nil {
		return err
	}
	if t.pool.inject.CompareAndSwap(true, false) {
		if t.pool.mode == "unknown" {
			t.pool.failRead.Store(true)
		}
		if t.pool.cancel != nil {
			t.pool.cancel()
		}
		return errors.New("injected connection EOF after server committed")
	}
	return nil
}

func TestSeasonUpgradeLostCommitAcknowledgement(t *testing.T) {
	video := upgradeTestVideo(t)
	for _, mode := range []string{"committed", "not committed", "unknown", "cancelled request"} {
		t.Run(mode, func(t *testing.T) {
			db := testutil.InitTestDB()
			root := t.TempDir()
			one := 1
			a := model.Anime{Title: "Review", EpisodeCount: &one, Status: model.AnimeStatusFinished}
			if err := db.Create(&a).Error; err != nil {
				t.Fatal(err)
			}
			season := BuildAnimeSavePath(root, &a)
			if err := os.MkdirAll(season, 0700); err != nil {
				t.Fatal(err)
			}
			oldPath := filepath.Join(season, "Review S01E01.mp4")
			if err := os.WriteFile(oldPath, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			oldID := "old"
			old := model.Download{TorrentID: oldID, Name: oldID, URL: "old", AnimeID: &a.ID, EpisodeNumber: &one, Status: model.DownloadStatusCompleted, FilePath: &oldPath}
			db.Create(&old)
			ep := model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: 1, Downloaded: true, DownloadID: &oldID, FilePath: &oldPath, QualityStatus: model.QualityUpgradeNeeded}
			db.Create(&ep)
			stage := BuildTorrentPackSavePath(root, a.ID, 1, 1, "PACK", "magnet")
			os.MkdirAll(stage, 0700)
			incoming := filepath.Join(stage, "ep-01.mkv")
			os.WriteFile(incoming, video, 0600)
			os.WriteFile(strings.TrimSuffix(incoming, filepath.Ext(incoming))+".chs.srt", []byte("1\n00:00:00,000 --> 00:00:02,000\n这是简体字幕\n"), 0600)
			d := model.Download{TorrentID: "upgrade", Name: "upgrade", URL: "magnet", AnimeID: &a.ID, EpisodeNumber: &one, EpisodeStart: &one, EpisodeEnd: &one, Scope: model.DownloadScopeSeason, Purpose: model.DownloadPurposeSeasonUpgrade, DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusDownloading, SavePath: &stage, SourceTitle: "[Review] Series [1080p][简体]"}
			db.Create(&d)
			svc := NewService(db, &config.Config{MediaRoot: root}, nil)
			svc.RegisterExecutor(model.DownloadTypeTorrent, &upgradeFileGate{pause: func() error { return nil }})
			conn, _ := db.DB()
			pool := &commitAckPool{ConnPool: conn, mode: mode}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled request" {
				pool.cancel = cancel
			}
			pool.inject.Store(true)
			before := db.Statement.ConnPool
			db.Statement.ConnPool = pool
			defer func() { db.Statement.ConnPool = before }()
			completed := svc.CompleteSeasonPack(ctx, d.ID, map[int]string{1: incoming}, root)
			if pool.inject.Load() {
				t.Fatal("test did not reach database commit")
			}
			if mode == "unknown" {
				journals, _ := filepath.Glob(filepath.Join(root, ".anidog-upgrades", "anime-*", "task-*", "*", "journal.json"))
				if completed || len(journals) != 1 {
					t.Fatal("unknown outcome must retain its recovery journal")
				}
				pool.failRead.Store(false)
				if svc.CompleteSeasonPack(context.Background(), d.ID, map[int]string{1: incoming}, root) {
					t.Fatal("unresolved swap repeated")
				}
			}
			if err := svc.RecoverSeasonUpgrades(context.Background()); err != nil {
				t.Fatal(err)
			}
			var got model.AnimeEpisode
			db.First(&got, ep.ID)
			var task model.Download
			db.First(&task, d.ID)
			if mode == "not committed" {
				if completed || task.Status != model.DownloadStatusPaused || got.FilePath == nil || *got.FilePath != oldPath {
					t.Fatalf("uncommitted swap not rolled back: %+v %+v", task, got)
				}
				if data, _ := os.ReadFile(oldPath); string(data) != "original" {
					t.Fatal("original lost")
				}
				return
			}
			if task.Status != model.DownloadStatusCompleted || task.BackupPath == "" {
				t.Fatalf("committed swap lost: %+v", task)
			}
			if got.DownloadID == nil || *got.DownloadID != d.TorrentID || got.QualityStatus != model.QualityMeetsTarget {
				t.Fatalf("committed episode mapping lost: %+v", got)
			}
			if data, _ := os.ReadFile(filepath.Join(task.BackupPath, filepath.Base(oldPath))); string(data) != "original" {
				t.Fatal("backup lost")
			}
			if got.FilePath == nil {
				t.Fatal("episode path cleared")
			}
			if _, err := os.Stat(*got.FilePath); err != nil {
				var task model.Download
				db.First(&task, d.ID)
				t.Fatalf("commit ACK loss: episode references missing new file %s; task status=%s; original restored=%v", *got.FilePath, task.Status, func() bool { _, e := os.Stat(oldPath); return e == nil }())
			}
		})
	}
}
