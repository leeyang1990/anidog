package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	dl "github.com/anidog/anidog-go/internal/service/download"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Opt-in real PostgreSQL contracts, also executed by the isolated media E2E.
// The database-name guard prevents fixture maintenance against AniDog's database.
func TestPostgresEvidenceRetention(t *testing.T) {
	if os.Getenv("ANIDOG_POSTGRES_CONTRACT") != "1" {
		t.Skip("requires an isolated PostgreSQL fixture")
	}
	db, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(6)
	var name string
	if err = db.Raw("SELECT current_database()").Scan(&name).Error; err != nil || name != "fixture" {
		t.Fatal("PostgreSQL fixture database required", name, err)
	}
	if err = db.AutoMigrate(&model.Download{}, &model.MediaEvidence{}, &model.AnimeEpisode{}); err != nil {
		t.Fatal(err)
	}
	t.Run("existing schema migration", func(t *testing.T) {
		// Isolate the pre-constraint schema and data in a rollback-only schema.
		rollback := errors.New("fixture rollback")
		err := db.Transaction(func(tx *gorm.DB) error {
			schema := fmt.Sprintf("legacy_contract_%d", time.Now().UnixNano())
			if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
				return err
			}
			if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
				return err
			}
			if err := tx.AutoMigrate(&model.Download{}, &model.MediaEvidence{}); err != nil {
				return err
			}
			for _, edge := range []struct {
				row  interface{}
				name string
			}{{&model.Download{}, "EvidenceOrigin"}, {&model.MediaEvidence{}, "OriginDownload"}} {
				if err := tx.Migrator().DropConstraint(edge.row, edge.name); err != nil {
					return err
				}
			}
			origin := model.Download{TorrentID: "legacy-origin", Name: "legacy", URL: "fixture"}
			if err := tx.Create(&origin).Error; err != nil {
				return err
			}
			child := model.Download{TorrentID: "legacy-child", Name: "legacy", URL: "fixture", EvidenceReusedFrom: &origin.ID}
			if err := tx.Create(&child).Error; err != nil {
				return err
			}
			proof := model.MediaEvidence{VideoHash: "legacy", InspectorVersion: "subtitle-v3", Evidence: `{"language":"simplified"}`, OriginDownloadID: &origin.ID}
			if err := tx.Create(&proof).Error; err != nil {
				return err
			}
			if err := tx.AutoMigrate(&model.Download{}, &model.MediaEvidence{}); err != nil {
				return err
			}
			var restored model.MediaEvidence
			if err := tx.First(&restored, proof.ID).Error; err != nil {
				return err
			}
			if restored.OriginDownloadID == nil || *restored.OriginDownloadID != origin.ID || restored.Evidence != proof.Evidence {
				return errors.New("migration altered existing provenance")
			}
			var restoredChild model.Download
			if err := tx.First(&restoredChild, child.ID).Error; err != nil {
				return err
			}
			if restoredChild.EvidenceReusedFrom == nil || *restoredChild.EvidenceReusedFrom != origin.ID {
				return errors.New("migration altered reuse origin")
			}
			for _, edge := range []struct {
				row  interface{}
				name string
			}{{&model.Download{}, "EvidenceOrigin"}, {&model.MediaEvidence{}, "OriginDownload"}} {
				if !tx.Migrator().HasConstraint(edge.row, edge.name) {
					return errors.New("migration omitted foreign key")
				}
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal("existing schema migration", err)
		}
	})
	prefix := fmt.Sprintf("retention-contract-%d-", time.Now().UnixNano())
	create := func(t *testing.T, suffix string) model.Download {
		row := model.Download{TorrentID: prefix + suffix, Name: suffix, URL: "isolated fixture", Status: model.DownloadStatusSuperseded}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&row).UpdateColumn("updated_at", time.Now().Add(-100*24*time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
		return row
	}
	t.Run("retention and release", func(t *testing.T) {
		root := create(t, "retained")
		expired := create(t, "expired")
		proof := model.MediaEvidence{VideoHash: prefix + "digest", InspectorVersion: "retention-contract", Evidence: "{}", OriginDownloadID: &root.ID}
		if err := db.Create(&proof).Error; err != nil {
			t.Fatal(err)
		}
		NewMaintenanceJob(db, 90*24*time.Hour).Run(context.Background())
		var n int64
		if err := db.Model(&model.Download{}).Where("id = ?", root.ID).Count(&n).Error; err != nil || n != 1 {
			t.Fatal("origin removed", n, err)
		}
		if err := db.Model(&model.Download{}).Where("id = ?", expired.ID).Count(&n).Error; err != nil || n != 0 {
			t.Fatal("unreferenced expired attempt retained", n, err)
		}
		if err := db.Delete(&root).Error; err == nil {
			t.Fatal("native FK bypassed")
		}
		if err := db.Delete(&proof).Error; err != nil {
			t.Fatal(err)
		}
		NewMaintenanceJob(db, 90*24*time.Hour).Run(context.Background())
		if err := db.Model(&model.Download{}).Where("id = ?", root.ID).Count(&n).Error; err != nil || n != 0 {
			t.Fatal("released origin retained", n, err)
		}
	})
	for _, commit := range []bool{false, true} {
		t.Run(fmt.Sprintf("concurrent delete commit=%t", commit), func(t *testing.T) {
			root := create(t, fmt.Sprint("concurrent-", commit))
			deleting := db.Begin()
			if deleting.Error != nil {
				t.Fatal(deleting.Error)
			}
			defer deleting.Rollback()
			if err := deleting.Delete(&root).Error; err != nil {
				t.Fatal(err)
			}
			ready := make(chan int, 1)
			finished := make(chan error, 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
					var pid int
					if err := tx.Raw("select pg_backend_pid()").Scan(&pid).Error; err != nil {
						return err
					}
					ready <- pid
					proof := model.MediaEvidence{VideoHash: prefix + fmt.Sprint("race-", commit), InspectorVersion: "retention-contract", Evidence: "{}", OriginDownloadID: &root.ID}
					return tx.Create(&proof).Error
				})
				finished <- err
			}()
			var pid int
			select {
			case pid = <-ready:
			case err := <-finished:
				t.Fatal("insert failed before locking", err)
			case <-time.After(5 * time.Second):
				t.Fatal("insert not started")
			}
			// Observe PostgreSQL's actual lock wait, rather than assuming a sleep is
			// enough to overlap the independent transactions.
			deadline := time.After(5 * time.Second)
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
		wait:
			for {
				var blocked bool
				if err := db.Raw("select coalesce(wait_event_type = 'Lock',false) from pg_stat_activity where pid = ?", pid).Scan(&blocked).Error; err != nil {
					t.Fatal(err)
				}
				if blocked {
					break wait
				}
				select {
				case err := <-finished:
					t.Fatal("insert ignored in-flight deletion", err)
				case <-deadline:
					t.Fatal("no native FK lock observed")
				case <-tick.C:
				}
			}
			var finishErr error
			if commit {
				finishErr = deleting.Commit().Error
			} else {
				finishErr = deleting.Rollback().Error
			}
			if finishErr != nil {
				t.Fatal(finishErr)
			}
			select {
			case err := <-finished:
				if commit {
					var state interface{ SQLState() string }
					if err == nil || !errors.As(err, &state) || state.SQLState() != "23503" {
						t.Fatal("deleted origin accepted", err)
					}
				} else if err != nil {
					t.Fatal("rolled-back deletion prevented valid reference", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("insert failed to settle")
			}
			var dangling int64
			if err := db.Raw("select count(*) from media_evidence e left join download d on d.id=e.origin_download_id where e.video_hash like ? and e.origin_download_id is not null and d.id is null", prefix+"%").Scan(&dangling).Error; err != nil || dangling != 0 {
				t.Fatal("dangling origin", dangling, err)
			}
		})
	}
	for _, failRemoval := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit remove excludes new references failure=%t", failRemoval), func(t *testing.T) {
			origin := create(t, fmt.Sprintf("removal-%t", failRemoval))
			if err := db.Model(&origin).UpdateColumn("download_type", model.DownloadTypeTorrent).Error; err != nil {
				t.Fatal(err)
			}
			probe := &originRemovalProbe{entered: make(chan struct{}), release: make(chan struct{})}
			if failRemoval {
				probe.err = errors.New("fixture downloader unavailable")
			}
			service := dl.NewService(db, &config.Config{}, nil)
			service.RegisterExecutor(model.DownloadTypeTorrent, probe)
			removed := make(chan error, 1)
			go func() { removed <- service.Remove(origin.ID, true) }()
			select {
			case <-probe.entered:
			case err := <-removed:
				t.Fatal("remove never reached executor", err)
			case <-time.After(5 * time.Second):
				t.Fatal("remove not started")
			}
			defer func() {
				select {
				case <-probe.release:
				default:
					close(probe.release)
				}
			}()
			ready, finished := make(chan int, 1), make(chan error, 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				finished <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
					var pid int
					if err := tx.Raw("select pg_backend_pid()").Scan(&pid).Error; err != nil {
						return err
					}
					ready <- pid
					return tx.Create(&model.MediaEvidence{VideoHash: prefix + fmt.Sprint("removal-", failRemoval), InspectorVersion: "contract", Evidence: "{}", OriginDownloadID: &origin.ID}).Error
				})
			}()
			var pid int
			select {
			case pid = <-ready:
			case err := <-finished:
				t.Fatal("insert failed before lock", err)
			case <-time.After(5 * time.Second):
				t.Fatal("insert not started")
			}
			until := time.Now().Add(5 * time.Second)
			for {
				var locked bool
				if err := db.Raw("select coalesce(wait_event_type = 'Lock',false) from pg_stat_activity where pid=?", pid).Scan(&locked).Error; err != nil {
					t.Fatal(err)
				}
				if locked {
					break
				}
				select {
				case err := <-finished:
					t.Fatal("reference inserted during filesystem effect", err)
				default:
				}
				if time.Now().After(until) {
					t.Fatal("origin lock not held through filesystem effect")
				}
				time.Sleep(10 * time.Millisecond)
			}
			close(probe.release)
			select {
			case err := <-removed:
				if (err != nil) != failRemoval {
					t.Fatal("unexpected removal result", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("remove not settled")
			}
			select {
			case err := <-finished:
				if failRemoval {
					if err != nil {
						t.Fatal("failed removal prevented valid reference", err)
					}
				} else {
					var state interface{ SQLState() string }
					if err == nil || !errors.As(err, &state) || state.SQLState() != "23503" {
						t.Fatal("deleted origin accepted", err)
					}
				}
			case <-time.After(5 * time.Second):
				t.Fatal("reference not settled")
			}
		})
	}
	// Remove fixture facts in dependency order. No file or production task is used.
	if err := db.Where("video_hash LIKE ?", prefix+"%").Delete(&model.MediaEvidence{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("torrent_id LIKE ?", prefix+"%").Delete(&model.Download{}).Error; err != nil {
		t.Fatal(err)
	}
}

// Embed unused operations so this probe only implements the relevant boundary.
type originRemovalProbe struct {
	dl.Executor
	entered chan struct{}
	release chan struct{}
	err     error
}

func (p *originRemovalProbe) Remove(string, bool) error { close(p.entered); <-p.release; return p.err }
