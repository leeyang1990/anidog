package download

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
)

// Run against retained real candidates in an isolated container with a read-only
// media mount. No production DB, tasks or original media are modified.
func TestStreamPreflightFixtures(t *testing.T) {
	var paths map[string]string
	if raw := os.Getenv("ANIDOG_PREFLIGHT_FIXTURES"); raw == "" {
		t.Skip("real-media fixtures not supplied")
	} else if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"720p", "traditional"} {
		t.Run(name, func(t *testing.T) {
			if paths[name] == "" {
				t.Fatal("missing fixture")
			}
			s := capacityService(t, 500*gib)
			row := model.Download{TorrentID: name, Name: name, URL: paths[name], Status: model.DownloadStatusDownloading}
			if err := s.db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			err := s.streamPreflight(row.ID, &Task{Source: SourceStream, SourceTitle: "Catalog claims [CHS]", EstimatedBytes: 2 * gib})(ctx, &stream.InterceptedVideo{URL: paths[name]})
			if err == nil || !strings.Contains(err.Error(), "流媒体预检不符合要求") {
				t.Fatalf("unsafe fixture admitted or wrong failure: %v", err)
			}
			kind, _ := classifyError(err, 0)
			if kind != model.FailureKindRejected {
				t.Fatal(kind)
			}
			if name == "720p" && !strings.Contains(err.Error(), "1280x720") {
				t.Fatal(err)
			}
			t.Log(err)
		})
	}
}

func TestRecoveryNotePreservesOriginalError(t *testing.T) {
	s := capacityService(t, 500*gib)
	aid, ep := uint(9), 12
	old := time.Now().Add(-time.Hour)
	row := model.Download{TorrentID: "history", Name: "history", URL: "old", AnimeID: &aid, EpisodeNumber: &ep, DownloadType: model.DownloadTypeStream, Status: model.DownloadStatusFailed, LastError: "视频拦截超时", UpdatedAt: old}
	s.db.Create(&row)
	s.db.Create(&model.OrchestratorDiagnosis{AnimeID: aid, EpisodeNumber: ep, SourceType: SourceStream, CheckedAt: time.Now(), Reason: "等待释放空间"})
	rows := []model.Download{row}
	s.annotateRecovery(context.Background(), rows)
	if rows[0].LastError != row.LastError || rows[0].RecoveryNote != "等待释放空间" {
		t.Fatal(rows)
	}
	var saved model.Download
	s.db.First(&saved, row.ID)
	if saved.RecoveryNote != "" || saved.LastError != row.LastError {
		t.Fatal("recovery presentation changed stored history")
	}
}
