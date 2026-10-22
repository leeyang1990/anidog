package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestAbandonmentIntentPrecedesEngineRemovalAndSurvivesProviderError(t *testing.T) {
	db := testutil.InitTestDB()
	hash := strings.Repeat("A", 40)
	row := model.Download{TorrentID: "dead", Name: "dead", URL: "magnet", InfoHash: &hash, DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusDownloading}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var saved model.Download
		db.First(&saved, row.ID)
		var count int64
		db.Model(&model.AbandonedTorrent{}).Where("info_hash = ?", hash).Count(&count)
		if saved.Status != model.DownloadStatusFailed || !saved.TorrentRemovalPending || count != 1 {
			t.Error("provider called before durable DB intent", saved, count)
		}
		if calls.Add(1) == 1 {
			http.Error(w, "temporarily unavailable", 503)
		}
	}))
	defer server.Close()
	s := &QBitSyncer{db: db, baseURL: server.URL, client: server.Client()}
	if !s.abandonDeadTorrent(context.Background(), &row, "无元数据，判死种") {
		t.Fatal("durable abandonment should release candidate slot")
	}
	var saved model.Download
	if err := db.First(&saved, row.ID).Error; err != nil || !saved.TorrentRemovalPending {
		t.Fatal("pending removal lost", err)
	}
	// Independent syncer represents recovery after process restart.
	restarted := &QBitSyncer{db: db, baseURL: server.URL, client: server.Client()}
	restarted.retryTorrentRemovals(context.Background())
	var count int64
	db.Model(&model.Download{}).Where("id = ?", row.ID).Count(&count)
	if count != 0 || calls.Load() != 2 {
		t.Fatal("removal did not converge", count, calls.Load())
	}
	db.Model(&model.AbandonedTorrent{}).Where("info_hash = ?", hash).Count(&count)
	if count != 1 {
		t.Fatal("blacklist lost during cleanup")
	}
}

func TestAbandonmentCannotDeleteAConcurrentWinner(t *testing.T) {
	db := testutil.InitTestDB()
	hash := strings.Repeat("B", 40)
	row := model.Download{TorrentID: "winner", Name: "winner", URL: "magnet", InfoHash: &hash, DownloadType: model.DownloadTypeTorrent, Status: model.DownloadStatusCompleted}
	db.Create(&row)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("stale patrol removed a completed torrent") }))
	defer server.Close()
	s := &QBitSyncer{db: db, baseURL: server.URL, client: server.Client()}
	if s.abandonDeadTorrent(context.Background(), &row, "stale metadata observation") {
		t.Fatal("stale abandonment admitted")
	}
	var saved model.Download
	db.First(&saved, row.ID)
	if saved.Status != model.DownloadStatusCompleted || saved.TorrentRemovalPending {
		t.Fatal(saved)
	}
	var count int64
	db.Model(&model.AbandonedTorrent{}).Count(&count)
	if count != 0 {
		t.Fatal("winner was blacklisted")
	}
}
