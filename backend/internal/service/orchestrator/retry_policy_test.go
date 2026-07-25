package orchestrator

import (
	"context"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestRetryCountForEpisodeIgnoresRejectedCandidate(t *testing.T) {
	db := testutil.InitTestDB()
	animeID, episode := uint(7), 3
	rows := []model.Download{
		{
			TorrentID: "wrong-season", Name: "wrong", URL: "magnet:?xt=urn:btih:AAA",
			Status: model.DownloadStatusFailed, DownloadType: model.DownloadTypeTorrent,
			AnimeID: &animeID, EpisodeNumber: &episode,
			FailureKind: model.FailureKindRejected, RetryCount: 3,
		},
		{
			TorrentID: "network-failure", Name: "network", URL: "https://example.test/video",
			Status: model.DownloadStatusFailed, DownloadType: model.DownloadTypeStream,
			AnimeID: &animeID, EpisodeNumber: &episode,
			FailureKind: model.FailureKindTransient, RetryCount: 1,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{db: db}
	if got := o.retryCountForEpisode(context.Background(), animeID, episode); got != 1 {
		t.Fatalf("got retry count %d, want 1", got)
	}
}

func TestSupersededInfoHashIsFilteredBeforeCandidateSelection(t *testing.T) {
	db := testutil.InitTestDB()
	animeID := uint(7)
	hash := "ABCDEF0123456789"
	row := model.Download{
		TorrentID: "old-candidate", Name: "old", URL: "magnet:?xt=urn:btih:" + hash,
		Status: model.DownloadStatusSuperseded, DownloadType: model.DownloadTypeTorrent,
		AnimeID: &animeID, InfoHash: &hash,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{db: db}
	if !o.hasAnimeInfoHashFailure(context.Background(), animeID, hash) {
		t.Fatal("superseded hash should be filtered so the next candidate can be selected")
	}
}

func TestFailedCandidateURLIsFilteredButNotReportedAsActiveBatch(t *testing.T) {
	db := testutil.InitTestDB()
	animeID := uint(7)
	candidateURL := "https://example.test/episode-03.torrent"
	row := model.Download{
		TorrentID: "failed-url", Name: "failed", URL: candidateURL,
		Status: model.DownloadStatusFailed, DownloadType: model.DownloadTypeTorrent,
		AnimeID: &animeID,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{db: db}
	if !o.hasAnimeURLFailure(context.Background(), animeID, candidateURL) {
		t.Fatal("failed candidate URL should be filtered before selecting top")
	}
	if o.isDuplicateURL(context.Background(), animeID, candidateURL) {
		t.Fatal("failed URL is not an active/completed batch")
	}
}
