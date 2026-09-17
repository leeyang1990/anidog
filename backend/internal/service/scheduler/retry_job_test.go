package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestRecoverySearchPreservesBudgetAndRearms(t *testing.T) {
	for _, scenario := range []string{"no candidate", "legacy null", "cancel during search", "unsubscribed", "archived", "completed during search", "replacement queued"} {
		t.Run(scenario, func(t *testing.T) {
			db := testutil.InitTestDB()
			anime := model.Anime{Title: scenario, IsSubscribed: scenario != "unsubscribed"}
			if scenario == "archived" {
				anime.MediaManagementState = "archived"
			}
			if err := db.Create(&anime).Error; err != nil {
				t.Fatal(err)
			}
			ep, due := 8, time.Now().Add(-time.Hour)
			row := model.Download{TorrentID: "failed", Name: "failed", URL: "failed", AnimeID: &anime.ID, EpisodeNumber: &ep,
				Status: model.DownloadStatusFailed, FailureKind: model.FailureKindTransient, RetryCount: 1, NextRetryAt: &due}
			if scenario == "legacy null" {
				row.NextRetryAt = nil
			}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			originalTime := row.UpdatedAt
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			job := NewRetryFailedJob(db, func(context.Context, *model.Anime) {
				calls++
				switch scenario {
				case "cancel during search":
					cancel()
				case "completed during search":
					if err := db.Model(&row).UpdateColumn("status", model.DownloadStatusSuperseded).Error; err != nil {
						t.Fatal(err)
					}
				case "replacement queued":
					next := row
					next.ID, next.TorrentID, next.Status, next.NextRetryAt = 0, "new", model.DownloadStatusPending, nil
					if err := db.Create(&next).Error; err != nil {
						t.Fatal(err)
					}
				}
			})
			job.Run(ctx)
			if err := db.First(&row, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if row.RetryCount != 1 || !row.UpdatedAt.Equal(originalTime) {
				t.Fatal("search consumed budget or changed failure timestamp")
			}
			if scenario == "completed during search" {
				if row.Status != model.DownloadStatusSuperseded || !row.NextRetryAt.Equal(due) {
					t.Fatal("completion overwritten")
				}
			} else if row.NextRetryAt == nil || !row.NextRetryAt.After(time.Now().Add(29*time.Minute)) {
				t.Fatal("search not rearmed")
			}
			want := 1
			if scenario == "unsubscribed" || scenario == "archived" {
				want = 0
			}
			if calls != want {
				t.Fatalf("calls=%d want=%d", calls, want)
			}
			job.Run(context.Background())
			if calls != want {
				t.Fatal("immediate repeat bypassed polling interval")
			}
		})
	}
}

func TestRecoveryLatestFailureAndUnprocessedBatch(t *testing.T) {
	db := testutil.InitTestDB()
	due, ep := time.Now().Add(-time.Hour), 1
	var rows []model.Download
	for _, name := range []string{"first", "second"} {
		anime := model.Anime{Title: name, IsSubscribed: true}
		if err := db.Create(&anime).Error; err != nil {
			t.Fatal(err)
		}
		row := model.Download{TorrentID: name, Name: name, URL: name, AnimeID: &anime.ID, EpisodeNumber: &ep,
			Status: model.DownloadStatusFailed, FailureKind: model.FailureKindTransient, NextRetryAt: &due}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	calls := 0
	job := NewRetryFailedJob(db, func(context.Context, *model.Anime) { calls++ })
	job.maxPerRound = 1
	job.Run(context.Background())
	if calls != 1 {
		t.Fatal("batch limit ignored")
	}
	if err := db.First(&rows[1], rows[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if !rows[1].NextRetryAt.Equal(due) {
		t.Fatal("unprocessed row modified")
	}
	newer := rows[1]
	newer.ID, newer.TorrentID, newer.FailureKind = 0, "newer", model.FailureKindExhausted
	if err := db.Create(&newer).Error; err != nil {
		t.Fatal(err)
	}
	job.Run(context.Background())
	if calls != 1 {
		t.Fatal("older transient bypassed latest exhausted attempt")
	}
}
