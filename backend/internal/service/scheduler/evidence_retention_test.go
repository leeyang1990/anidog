package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestMaintenanceKeepsReferencedAttempts(t *testing.T) {
	for _, root := range []string{"evidence", "reused attempt", "episode attempt", "episode hash", "upgrade backup", "unreferenced"} {
		t.Run(root, func(t *testing.T) {
			db := testutil.InitTestDB()
			origin := model.Download{TorrentID: "origin", Name: "origin", URL: "fixture", Status: model.DownloadStatusSuperseded}
			if root == "episode hash" {
				h := "ABC123"
				origin.InfoHash = &h
			}
			if root == "upgrade backup" {
				path := "/library/.anidog-upgrades/previous-episode"
				origin.BackupPath = path
			}
			if err := db.Create(&origin).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&origin).UpdateColumn("updated_at", time.Now().Add(-100*24*time.Hour)).Error; err != nil {
				t.Fatal(err)
			}
			var evidence model.MediaEvidence
			switch root {
			case "evidence":
				evidence = model.MediaEvidence{VideoHash: "digest", InspectorVersion: "subtitle-v3", Evidence: `{"language":"simplified"}`, OriginDownloadID: &origin.ID}
				if err := db.Create(&evidence).Error; err != nil {
					t.Fatal(err)
				}
			case "reused attempt":
				child := model.Download{TorrentID: "child", Name: "child", URL: "fixture", Status: model.DownloadStatusCompleted, EvidenceReusedFrom: &origin.ID}
				if err := db.Create(&child).Error; err != nil {
					t.Fatal(err)
				}
			case "episode attempt", "episode hash":
				identity := origin.TorrentID
				if root == "episode hash" {
					identity = *origin.InfoHash
				}
				ep := model.AnimeEpisode{AnimeID: 1, EpisodeNumber: 1, DownloadID: &identity, Downloaded: true}
				if err := db.Create(&ep).Error; err != nil {
					t.Fatal(err)
				}
			}
			NewMaintenanceJob(db, 90*24*time.Hour).Run(context.Background())
			var n int64
			if err := db.Model(&model.Download{}).Where("id = ?", origin.ID).Count(&n).Error; err != nil {
				t.Fatal(err)
			}
			want := int64(1)
			if root == "unreferenced" {
				want = 0
			}
			if n != want {
				t.Fatalf("root %s: retained %d, want %d", root, n, want)
			}
			if root == "evidence" {
				if err := db.Delete(&evidence).Error; err != nil {
					t.Fatal(err)
				}
				NewMaintenanceJob(db, 90*24*time.Hour).Run(context.Background())
				if err := db.Model(&model.Download{}).Where("id = ?", origin.ID).Count(&n).Error; err != nil || n != 0 {
					t.Fatal("released evidence still pins attempt", n, err)
				}
			}
		})
	}
}

func TestEvidenceForeignKeysProtectBothProvenanceEdges(t *testing.T) {
	for _, kind := range []string{"content", "reuse"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.InitTestDB()
			if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
				t.Fatal(err)
			}
			origin := model.Download{TorrentID: "origin", Name: "origin", URL: "fixture", Status: model.DownloadStatusSuperseded}
			if err := db.Create(&origin).Error; err != nil {
				t.Fatal(err)
			}
			if kind == "content" {
				d := model.MediaEvidence{VideoHash: "digest", InspectorVersion: "v", Evidence: "{}", OriginDownloadID: &origin.ID}
				if err := db.Create(&d).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				d := model.Download{TorrentID: "child", Name: "child", URL: "fixture", EvidenceReusedFrom: &origin.ID}
				if err := db.Create(&d).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Bypass the scheduler to verify the database invariant, not its WHERE clause.
			if err := db.Delete(&origin).Error; err == nil {
				t.Fatal("native FK allowed deleting a provenance root")
			}
			missing := uint(999999)
			if kind == "content" {
				d := model.MediaEvidence{VideoHash: "missing", InspectorVersion: "v", Evidence: "{}", OriginDownloadID: &missing}
				if err := db.Create(&d).Error; err == nil {
					t.Fatal("missing content origin accepted")
				}
			} else {
				d := model.Download{TorrentID: "missing", Name: "missing", URL: "fixture", EvidenceReusedFrom: &missing}
				if err := db.Create(&d).Error; err == nil {
					t.Fatal("missing reuse origin accepted")
				}
			}
		})
	}
}
