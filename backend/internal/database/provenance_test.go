//go:build cgo || desktop

package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"gorm.io/gorm"
)

func TestSQLiteConnectionsEnforceProvenance(t *testing.T) {
	db, err := openSQLite(filepath.Join(t.TempDir(), "contract.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pool.SetMaxOpenConns(2)
	first, err := pool.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := pool.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, conn := range []*sql.Conn{first, second} {
		var enabled int
		if err := conn.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
			t.Fatal("connection has no foreign-key protection", enabled, err)
		}
	}
	first.Close()
	second.Close()
	if err := db.AutoMigrate(&model.Download{}, &model.MediaEvidence{}); err != nil {
		t.Fatal(err)
	}
	origin := model.Download{TorrentID: "origin", Name: "origin", URL: "fixture"}
	if err := db.Create(&origin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.MediaEvidence{VideoHash: "digest", InspectorVersion: "v", Evidence: "{}", OriginDownloadID: &origin.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&origin).Error; err == nil {
		t.Fatal("driver allowed deleting referenced provenance")
	}
}
