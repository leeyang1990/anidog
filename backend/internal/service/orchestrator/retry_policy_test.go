package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	if got := o.retryCountForEpisode(context.Background(), animeID, episode); got != 2 {
		t.Fatalf("got next attempt count %d, want 2", got)
	}
}

func TestRecoveryBudgetIsConsumedOnlyByNewAttempts(t *testing.T) {
	db := testutil.InitTestDB()
	o := &Orchestrator{db: db}
	aid, ep := uint(1), 1
	ctx := context.Background()
	if got := o.retryCountForEpisode(ctx, aid, ep); got != 0 {
		t.Fatal("initial attempt must be zero")
	}
	for attempt := 0; attempt < 5; attempt++ {
		row := model.Download{TorrentID: fmt.Sprintf("attempt-%d", attempt), Name: "attempt", URL: "attempt", AnimeID: &aid, EpisodeNumber: &ep,
			Status: model.DownloadStatusFailed, FailureKind: model.FailureKindTransient, RetryCount: attempt}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		want := min(attempt+1, 3)
		for i := 0; i < 2; i++ {
			if got := o.retryCountForEpisode(ctx, aid, ep); got != want {
				t.Fatalf("got=%d want=%d", got, want)
			}
		}
	}
}

func TestOldSearchScheduleDoesNotOverrideNewerFailure(t *testing.T) {
	db := testutil.InitTestDB()
	aid, ep := uint(1), 1
	future, due := time.Now().Add(time.Hour), time.Now().Add(-time.Minute)
	for i, next := range []*time.Time{&future, &due} {
		row := model.Download{TorrentID: fmt.Sprintf("failed-%d", i), Name: "failed", URL: "failed", AnimeID: &aid, EpisodeNumber: &ep,
			Source: SourceStream, Status: model.DownloadStatusFailed, FailureKind: model.FailureKindTransient, NextRetryAt: next}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	o := &Orchestrator{db: db}
	if o.isDuplicate(context.Background(), aid, ep, SourceStream) {
		t.Fatal("older polling schedule blocked newer due attempt")
	}
}

func TestPausedSeasonPackPreservesEpisodeCoverage(t *testing.T) {
	db := testutil.InitTestDB()
	aid, start, end := uint(1), 1, 12
	pack := model.Download{TorrentID: "pack", Name: "pack", URL: "pack", AnimeID: &aid,
		Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end,
		Status: model.DownloadStatusPaused, SeekingAlternative: true}
	if err := db.Create(&pack).Error; err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{db: db}
	covered := o.downloadedEpisodes(context.Background(), aid)
	for ep := start; ep <= end; ep++ {
		if !covered[ep] {
			t.Fatalf("paused pack released episode %d", ep)
		}
	}
}

func TestSlowTorrentDoesNotBlockAlternativeCandidate(t *testing.T) {
	db := testutil.InitTestDB()
	animeID := uint(7)
	episodeSlow, episodeNormal := 2, 3
	rows := []model.Download{
		{
			TorrentID: "slow-active", Name: "slow", URL: "magnet:?xt=urn:btih:SLOW",
			Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeTorrent,
			Source: SourceBT, AnimeID: &animeID, EpisodeNumber: &episodeSlow, SeekingAlternative: true,
		},
		{
			TorrentID: "mikan-active", Name: "mikan", URL: "magnet:?xt=urn:btih:MIKAN",
			Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeTorrent,
			Source: SourceMikan, AnimeID: &animeID, EpisodeNumber: &episodeSlow,
		},
		{
			TorrentID: "normal-active", Name: "normal", URL: "magnet:?xt=urn:btih:NORMAL",
			Status: model.DownloadStatusDownloading, DownloadType: model.DownloadTypeTorrent,
			Source: SourceBT, AnimeID: &animeID, EpisodeNumber: &episodeNormal,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{db: db}
	covered := o.downloadedEpisodes(context.Background(), animeID)
	if !covered[episodeSlow] {
		t.Fatal("normal mikan candidate should cover the episode while slow bt keeps racing")
	}
	if !covered[episodeNormal] {
		t.Fatal("normal active torrent must continue to cover its episode")
	}
	if o.isDuplicate(context.Background(), animeID, episodeSlow, SourceBT) {
		t.Fatal("slow bt must release only the bt source slot")
	}
	if !o.isDuplicate(context.Background(), animeID, episodeSlow, SourceMikan) {
		t.Fatal("active mikan candidate must keep the independent mikan source slot")
	}
}

func TestTransientSlowHashUsesShortHalfOpenCooldown(t *testing.T) {
	db := testutil.InitTestDB()
	oldSlow := model.AbandonedTorrent{
		InfoHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Reason:   "4h 长期平均速度 1.0 KiB/s，判定不可用慢种",
		Kind:     model.FailureKindTransient, AbandonedAt: time.Now().Add(-13 * time.Hour),
	}
	recentSlow := model.AbandonedTorrent{
		InfoHash: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		Reason:   "长期平均速度过低，判定不可用慢种",
		Kind:     model.FailureKindTransient, AbandonedAt: time.Now().Add(-time.Hour),
	}
	oldDead := model.AbandonedTorrent{
		InfoHash: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
		Reason:   "metaDL 超时无元数据，判死种",
		Kind:     model.FailureKindTransient, AbandonedAt: time.Now().Add(-13 * time.Hour),
	}
	if err := db.Create(&[]model.AbandonedTorrent{oldSlow, recentSlow, oldDead}).Error; err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{db: db}
	if o.hasHistoricalFailure(context.Background(), oldSlow.InfoHash) {
		t.Fatal("slow hash should enter half-open after 12 hours")
	}
	if !o.hasHistoricalFailure(context.Background(), recentSlow.InfoHash) {
		t.Fatal("recent slow hash must remain cooling")
	}
	if !o.hasHistoricalFailure(context.Background(), oldDead.InfoHash) {
		t.Fatal("confirmed dead hash should keep the longer cooldown")
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
