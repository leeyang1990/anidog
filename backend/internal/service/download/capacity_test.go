package download

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	puresqlite "github.com/glebarez/sqlite"
	"github.com/shirou/gopsutil/v3/disk"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func capacityService(t *testing.T, free int64) *Service {
	t.Helper()
	db, err := gorm.Open(puresqlite.Open(fmt.Sprintf("file:capacity-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Setting{}, &model.Download{}, &model.Anime{}, &model.AnimeEpisode{}, &model.OrchestratorDiagnosis{}); err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	db.Create(&model.Setting{Key: "media.min_free_gb", Value: "100"})
	db.Create(&model.Setting{Key: "media.max_used_percent", Value: "95"})
	s := NewService(db, &config.Config{MediaRoot: t.TempDir()}, nil)
	s.capacityProbe = func(string) (*disk.UsageStat, error) {
		return &disk.UsageStat{Free: uint64(free), UsedPercent: 99}, nil
	}
	return s
}

func TestCapacityUsesRemainingBytesInsteadOfPercent(t *testing.T) {
	s := capacityService(t, 200*gib)
	total, done := int64(140*gib), int64(80*gib)
	s.db.Create(&model.Download{TorrentID: "queued", Name: "queued", URL: "magnet:x", Status: model.DownloadStatusPaused, TotalBytes: &total, DownloadedBytes: &done})
	if err := s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 40 * gib}, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 41 * gib}, 0); err == nil {
		t.Fatal("queued remaining bytes and reserve must both count")
	}
	if err := s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 80 * gib}, 1); err != nil {
		t.Fatal("a resized task must not count itself twice:", err)
	}
	s.db.Model(&model.Setting{}).Where("key = ?", "media.min_free_gb").Update("value", "0")
	if err := s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 140 * gib}, 0); err != nil {
		t.Fatal("zero must disable the extra reserve:", err)
	}
	if err := s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 141 * gib}, 0); err == nil {
		t.Fatal("zero reserve must still prevent overbooking actual free space")
	}
}

func TestConcurrentCapacityAdmissionReservesOnlyOnce(t *testing.T) {
	s := capacityService(t, 112*gib)
	executor := &pathCaptureExecutor{paths: make(chan string, 2)}
	s.RegisterExecutor(model.DownloadTypeTorrent, executor)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, name := range []string{"one", "two"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			_, err := s.Create(context.Background(), &Task{Name: name, URL: "magnet:" + name, Source: SourceManual, DownloadType: model.DownloadTypeTorrent, EstimatedBytes: 8 * gib})
			results <- err
		}(name)
	}
	wg.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d simultaneous tasks with room for only one", accepted)
	}
	select {
	case <-executor.paths:
	case <-time.After(time.Second):
		t.Fatal("admitted task did not execute")
	}
}

func TestLargerMagnetMetadataPausesWithoutDeletingPayload(t *testing.T) {
	s := capacityService(t, 115*gib)
	s.RegisterExecutor(model.DownloadTypeTorrent, &pathCaptureExecutor{})
	size := int64(4 * gib)
	row := model.Download{TorrentID: "larger-metadata", Name: "larger", URL: "magnet:x", DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusDownloading, TotalBytes: &size}
	s.db.Create(&row)
	syncer := &QBitSyncer{db: s.db, raceService: s}
	info := map[string]interface{}{"size": float64(20 * gib), "downloaded": float64(0), "progress": float64(0)}
	if !syncer.holdOversizedTorrent(context.Background(), &row, info, map[string]interface{}{}) {
		t.Fatal("larger metadata ignored the reserve")
	}
	var saved model.Download
	s.db.First(&saved, row.ID)
	if saved.Status != model.DownloadStatusPaused || saved.TotalBytes == nil || *saved.TotalBytes != 20*gib || saved.QualityNote == "" {
		t.Fatal(saved)
	}
}

func TestCapacitySeparatesVolumesIncludingNestedMount(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			s := capacityService(t, 150*gib)
			nested := filepath.Join(s.cfg.MediaRoot, "nested-mount")
			if err := os.MkdirAll(nested, 0700); err != nil {
				t.Fatal(err)
			}
			s.capacityVolume = func(path string) (string, error) {
				if path == nested && !same {
					return "other-volume", nil
				}
				return "target-volume", nil
			}
			bytes := int64(40 * gib)
			if err := s.db.Create(&model.Download{TorrentID: "nested", Name: "nested", URL: "magnet:x", Status: model.DownloadStatusPaused, SavePath: &nested, TotalBytes: &bytes}).Error; err != nil {
				t.Fatal(err)
			}
			err := s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 20 * gib}, 0)
			if (err != nil) != same {
				t.Fatalf("same volume=%v: %v", same, err)
			}
			s.capacityVolume = func(string) (string, error) { return "", errors.New("mount unavailable") }
			if s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 1}, 0) == nil {
				t.Fatal("unknown volume must not admit work")
			}
		})
	}
}

func TestSharedFilesystemCapacity(t *testing.T) {
	s := capacityService(t, 150*gib)
	sibling := filepath.Join(filepath.Dir(s.cfg.MediaRoot), "movies")
	if err := os.MkdirAll(sibling, 0700); err != nil {
		t.Fatal(err)
	}
	bytes := int64(40 * gib)
	d := model.Download{TorrentID: "other-directory", Name: "other", URL: "magnet:x", Status: model.DownloadStatusDownloading, SavePath: &sibling, TotalBytes: &bytes}
	if err := s.db.Create(&d).Error; err != nil {
		t.Fatal(err)
	}
	err := s.ensureTaskCapacity(context.Background(), &Task{SavePath: s.cfg.MediaRoot, EstimatedBytes: 20 * gib}, 0)
	if err == nil {
		t.Fatal("same volume: admitted 20 GiB with 150 GiB free, 40 GiB pending and 100 GiB reserve; sibling directory was excluded")
	}
}

func TestCapacityInvalidReserveNeverBecomesZero(t *testing.T) {
	s := capacityService(t, 150*gib)
	for _, value := range []string{"invalid", "NaN", "+Inf", "-1"} {
		if err := s.db.Model(&model.Setting{}).Where("key = ?", "media.min_free_gb").Update("value", value).Error; err != nil {
			t.Fatal(err)
		}
		if err := s.ensureTaskCapacity(context.Background(), &Task{EstimatedBytes: 1}, 0); err == nil {
			t.Fatalf("invalid reserve admitted a task: %s", value)
		}
	}
}
