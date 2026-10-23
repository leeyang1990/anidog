package download

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
)

func TestStreamPreflightChecksReferenceBeforeLanguageOrManualFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "short.mkv")
	if err := os.WriteFile(path, upgradeTestVideo(t), 0600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{SourceStream, SourceManual} {
		t.Run(source, func(t *testing.T) {
			s := capacityService(t, 500*gib)
			a := model.Anime{Title: "Reference episode"}
			if err := s.db.Create(&a).Error; err != nil {
				t.Fatal(err)
			}
			ep := 1
			if err := s.db.Create(&model.AnimeEpisode{AnimeID: a.ID, EpisodeNumber: ep, DurationSeconds: 60}).Error; err != nil {
				t.Fatal(err)
			}
			row := model.Download{TorrentID: "short", Name: "short", URL: path, Status: model.DownloadStatusDownloading, VerifiedLanguages: `["simplified"]`}
			if err := s.db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			err := s.streamPreflight(row.ID, &Task{Source: source, AnimeID: &a.ID, EpisodeNumber: &ep})(context.Background(), &stream.InterceptedVideo{URL: path})
			if err == nil || !strings.Contains(err.Error(), "剧集参考 60 秒") {
				t.Fatalf("short video passed the reference guard: %v", err)
			}
			var saved model.Download
			if err := s.db.First(&saved, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if saved.VerifiedLanguages != "" || saved.TotalBytes != nil {
				t.Fatal("rejected reference retained old subtitle evidence or reserved payload bytes")
			}
		})
	}
}

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
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			err := s.streamPreflight(row.ID, &Task{Source: SourceStream, SourceTitle: "Catalog claims [CHS]", EstimatedBytes: 2 * gib})(ctx, &stream.InterceptedVideo{URL: paths[name]})
			if name == "traditional" && err == nil {
				s.cfg.VerifyDownloadedMedia = true
				row.Source, row.DownloadType = SourceStream, model.DownloadTypeStream
				err = s.validateCompletedMedia(ctx, &row, 1, paths[name])
			}
			if err == nil {
				t.Fatal("unsafe fixture admitted")
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
