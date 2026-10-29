//go:build cgo || desktop

package database

import (
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"gorm.io/gorm"
)

func TestSQLiteExistingProvenanceMigration(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		name := "valid"
		if invalid {
			name = "invalid"
		}
		t.Run(name, func(t *testing.T) {
			db, err := openSQLite(filepath.Join(t.TempDir(), "legacy.db"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
			if err != nil {
				t.Fatal(err)
			}
			pool, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			pool.SetMaxOpenConns(1)
			if err := db.AutoMigrate(&model.Download{}, &model.MediaEvidence{}); err != nil {
				t.Fatal(err)
			}
			origin := model.Download{TorrentID: "old-origin", Name: "old", URL: "fixture"}
			if err := db.Create(&origin).Error; err != nil {
				t.Fatal(err)
			}
			child := model.Download{TorrentID: "old-child", Name: "old", URL: "fixture", EvidenceReusedFrom: &origin.ID}
			if err := db.Create(&child).Error; err != nil {
				t.Fatal(err)
			}
			originID := origin.ID
			if invalid {
				originID += 1000
			}
			proof := model.MediaEvidence{VideoHash: "digest", InspectorVersion: "subtitle-v3", Evidence: `{"language":"simplified"}`, OriginDownloadID: &originID}
			if err := db.Create(&proof).Error; err != nil {
				t.Fatal(err)
			}
			db.Config.DisableForeignKeyConstraintWhenMigrating = false
			err = migrateSchema(db, &model.Download{}, &model.MediaEvidence{})
			if (err != nil) != invalid {
				t.Fatal("unexpected migration outcome", err)
			}
			var enabled int
			if err := db.Raw("PRAGMA foreign_keys").Scan(&enabled).Error; err != nil || enabled != 1 {
				t.Fatal("connection enforcement lost", enabled, err)
			}
			var restored model.MediaEvidence
			if err := db.First(&restored, proof.ID).Error; err != nil {
				t.Fatal(err)
			}
			if restored.OriginDownloadID == nil || *restored.OriginDownloadID != originID || restored.Evidence != proof.Evidence {
				t.Fatal("migration altered proof")
			}
			var restoredChild model.Download
			if err := db.First(&restoredChild, child.ID).Error; err != nil {
				t.Fatal(err)
			}
			if restoredChild.EvidenceReusedFrom == nil || *restoredChild.EvidenceReusedFrom != origin.ID {
				t.Fatal("migration altered reuse")
			}
			has := db.Migrator().HasConstraint(&model.MediaEvidence{}, "OriginDownload")
			if has == invalid {
				t.Fatal("schema change was not atomic", has, invalid)
			}
			if !invalid {
				if err := db.Delete(&origin).Error; err == nil {
					t.Fatal("origin not protected")
				}
				if err := migrateSchema(db, &model.Download{}, &model.MediaEvidence{}); err != nil {
					t.Fatal("migration not idempotent", err)
				}
			}
		})
	}
}
