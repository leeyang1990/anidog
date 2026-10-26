package download

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

type receiptExecutor struct {
	pathCaptureExecutor
	hash string
}

func (e *receiptExecutor) Execute(context.Context, *Task, ProgressCallback) (*Result, error) {
	return &Result{TorrentID: e.hash}, nil
}

func TestTorrentSubmissionPersistsProviderIdentityWithoutCompleting(t *testing.T) {
	for _, url := range []string{"https://fixture.invalid/download?id=7", "magnet:?xt=urn:btih:" + strings.Repeat("B", 40)} {
		t.Run(url, func(t *testing.T) {
			db := testutil.InitTestDB()
			s := NewService(db, &config.Config{}, nil)
			hash := strings.Repeat("a", 40)
			s.RegisterExecutor(model.DownloadTypeTorrent, &receiptExecutor{hash: hash})
			row := model.Download{TorrentID: "runtime-only", Name: "receipt", URL: url, DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusPending}
			if hint := ExtractInfoHash(url); hint != "" {
				row.InfoHash = &hint
			}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			s.execute(row.ID, row.TorrentID, &Task{URL: url, DownloadType: model.DownloadTypeTorrent})
			var saved model.Download
			if err := db.First(&saved, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if saved.InfoHash == nil || *saved.InfoHash != strings.ToUpper(hash) || saved.TorrentID != hash || saved.Status != model.DownloadStatusDownloading || saved.CompletedAt != nil {
				t.Fatalf("submission lost confirmed identity or became completion: %+v", saved)
			}
		})
	}
}

func TestSyncRestoresReceiptIdentityOnlyFromObservedEngineTask(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		observed     bool
		adopted      bool
	}{
		{"active", model.DownloadStatusDownloading, true, true},
		{"paused", model.DownloadStatusPaused, true, true},
		{"absent", model.DownloadStatusDownloading, false, false},
		{"terminal", model.DownloadStatusSuperseded, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.InitTestDB()
			hash := strings.Repeat("a", 40)
			row := model.Download{TorrentID: hash, Name: "old receipt", URL: "https://fixture.invalid/download?id=7", DownloadType: model.DownloadTypeTorrent, Status: tc.status}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/auth/login", "/api/v2/app/setPreferences":
					w.WriteHeader(http.StatusNoContent)
				case "/api/v2/torrents/info":
					rows := []map[string]interface{}{}
					if tc.observed {
						state := "downloading"
						if tc.status == model.DownloadStatusPaused {
							state = "pausedDL"
						}
						rows = append(rows, map[string]interface{}{"hash": hash, "state": state, "has_metadata": true, "progress": .5, "size": float64(200), "downloaded": float64(100)})
					}
					_ = json.NewEncoder(w).Encode(rows)
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer server.Close()
			s := &QBitSyncer{db: db, baseURL: server.URL, client: server.Client()}
			if err := s.Sync(context.Background()); err != nil {
				t.Fatal(err)
			}
			var saved model.Download
			if err := db.First(&saved, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if (saved.InfoHash != nil) != tc.adopted || saved.Status != tc.status {
				t.Fatalf("invalid receipt recovery: %+v", saved)
			}
			if tc.adopted && (*saved.InfoHash != strings.ToUpper(hash) || saved.Progress != 50) {
				t.Fatalf("observed transfer was not synchronized: %+v", saved)
			}
		})
	}
}
