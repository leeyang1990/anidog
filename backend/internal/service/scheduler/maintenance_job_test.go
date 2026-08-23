package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestMaintenanceJobOnlyDeletesExpiredSupersededDownloads(t *testing.T) {
	db := testutil.InitTestDB()
	rows := []model.Download{
		{TorrentID: "old-superseded", Name: "old", URL: "old", Status: model.DownloadStatusSuperseded},
		{TorrentID: "new-superseded", Name: "new", URL: "new", Status: model.DownloadStatusSuperseded},
		{TorrentID: "old-completed", Name: "completed", URL: "completed", Status: model.DownloadStatusCompleted},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	if err := db.Model(&model.Download{}).
		Where("torrent_id IN ?", []string{"old-superseded", "old-completed"}).
		UpdateColumn("updated_at", old).Error; err != nil {
		t.Fatal(err)
	}

	NewMaintenanceJob(db, 90*24*time.Hour).Run(context.Background())

	var remaining []model.Download
	if err := db.Order("torrent_id").Find(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining rows = %d, want 2: %#v", len(remaining), remaining)
	}
	for _, row := range remaining {
		if row.TorrentID == "old-superseded" {
			t.Fatal("expired superseded row was not deleted")
		}
	}
}
