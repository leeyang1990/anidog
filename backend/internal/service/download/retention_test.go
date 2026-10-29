package download

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
)

type retainedOriginExecutor struct {
	pathCaptureExecutor
	removals int
}

func (e *retainedOriginExecutor) Remove(string, bool) error { e.removals++; return nil }

func TestExplicitRemovalProtectsEvidenceBeforeDownloaderSideEffects(t *testing.T) {
	for _, kind := range []string{"content", "reuse"} {
		t.Run(kind, func(t *testing.T) {
			s := capacityService(t, 500*gib)
			path := filepath.Join(t.TempDir(), "original.mp4")
			if err := os.WriteFile(path, []byte("preserved"), 0600); err != nil {
				t.Fatal(err)
			}
			row := model.Download{TorrentID: "origin", Name: "origin", URL: "fixture", FilePath: &path, DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusSuperseded}
			if err := s.db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			if kind == "content" {
				if err := s.db.Create(&model.MediaEvidence{VideoHash: "digest", InspectorVersion: "v", Evidence: "{}", OriginDownloadID: &row.ID}).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				if err := s.db.Create(&model.Download{TorrentID: "child", Name: "child", URL: "fixture", EvidenceReusedFrom: &row.ID}).Error; err != nil {
					t.Fatal(err)
				}
			}
			executor := &retainedOriginExecutor{}
			s.RegisterExecutor(model.DownloadTypeTorrent, executor)
			if err := s.Remove(row.ID, true); !errors.Is(err, ErrDownloadReferenced) {
				t.Fatal("reference deletion was not rejected", err)
			}
			if executor.removals != 0 {
				t.Fatal("downloader side effect preceded the reference guard")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != "preserved" {
				t.Fatal("original files changed", err)
			}
		})
	}
}
