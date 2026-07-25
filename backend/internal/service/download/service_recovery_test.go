package download

import (
	"context"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestNormalizeHistoricalFailureKindsRejectsSeasonMismatch(t *testing.T) {
	db := testutil.InitTestDB()
	nextRetry := time.Now().Add(time.Hour)
	wrongBatchSize := int64(45_210_546_716)
	rows := []model.Download{
		{
			TorrentID:    "historical-season-mismatch",
			Name:         "无职转生 第3季 - 第03集",
			URL:          "magnet:?xt=urn:btih:ABC",
			Status:       model.DownloadStatusFailed,
			DownloadType: model.DownloadTypeTorrent,
			FailureKind:  model.FailureKindTransient,
			LastError:    "季度不匹配：错误选中第二季合集，已自动删除",
			NextRetryAt:  &nextRetry,
			TotalBytes:   &wrongBatchSize,
		},
		{
			TorrentID:    "historical-orphan",
			Name:         "旧版本已覆盖原始错误的任务",
			URL:          "magnet:?xt=urn:btih:DEF",
			Status:       model.DownloadStatusFailed,
			DownloadType: model.DownloadTypeTorrent,
			FailureKind:  model.FailureKindTransient,
			LastError:    "qBittorrent 中不存在对应任务，可能已被删除或下载器状态丢失",
			NextRetryAt:  &nextRetry,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	s := &Service{db: db}
	s.normalizeHistoricalFailureKinds(context.Background())

	for _, row := range rows {
		var saved model.Download
		if err := db.First(&saved, row.ID).Error; err != nil {
			t.Fatal(err)
		}
		if saved.FailureKind != model.FailureKindRejected {
			t.Fatalf("row %d: got failure kind %q, want rejected", row.ID, saved.FailureKind)
		}
		if saved.NextRetryAt != nil {
			t.Fatalf("row %d: historical rejected candidate retained cooldown: %v", row.ID, saved.NextRetryAt)
		}
		if saved.TotalBytes != nil {
			t.Fatalf("row %d: historical rejected candidate retained stale size: %v", row.ID, saved.TotalBytes)
		}
	}
}
