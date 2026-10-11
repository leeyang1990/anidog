package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestEpisodeStatusSeasonPack(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   string
		archived int
		missing  bool
	}{
		{"completed", model.DownloadStatusCompleted, 24, false},
		{"partial archive", model.DownloadStatusCompleted, 20, false},
		{"no archived files", model.DownloadStatusCompleted, 0, false},
		{"downloading", model.DownloadStatusDownloading, 0, false},
		{"pending", model.DownloadStatusPending, 0, false},
		{"paused", model.DownloadStatusPaused, 0, false},
		{"missing media", model.DownloadStatusCompleted, 24, true},
		{"superseded", model.DownloadStatusSuperseded, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.InitTestDB()
			count, trigger, start, end := 24, 1, 1, 24
			anime := model.Anime{Title: "幸运星", EpisodeCount: &count}
			if err := db.Create(&anime).Error; err != nil {
				t.Fatal(err)
			}
			pack := model.Download{TorrentID: "pack", Name: "幸运星 - 合集 01-24", URL: "magnet", AnimeID: &anime.ID,
				Scope: model.DownloadScopeSeason, EpisodeNumber: &trigger, EpisodeStart: &start, EpisodeEnd: &end,
				Status: tc.status, MediaMissing: tc.missing, Source: "mikan"}
			if err := db.Create(&pack).Error; err != nil {
				t.Fatal(err)
			}
			for ep := 1; ep <= tc.archived; ep++ {
				path := fmt.Sprintf("/media/幸运星 S01E%02d.mkv", ep)
				row := model.AnimeEpisode{AnimeID: anime.ID, EpisodeNumber: ep, Downloaded: true, DownloadID: &pack.TorrentID, FilePath: &path}
				if err := db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			router := testutil.SetupRouter()
			NewOrchestratorHandler(db, nil, nil).RegisterRoutes(router.Group("/api/v1"))
			w := testutil.MakeRequest(router, http.MethodGet, fmt.Sprintf("/api/v1/anime/%d/episode-status", anime.ID), nil, "")
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d: %s", w.Code, w.Body.String())
			}
			var response struct {
				Episodes []struct {
					EpisodeNumber int    `json:"episode_number"`
					Status        string `json:"status"`
					Source        string `json:"source"`
					DownloadID    uint   `json:"download_id"`
					FilePath      string `json:"file_path"`
				} `json:"episodes"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Episodes) != count {
				t.Fatalf("got %d episodes", len(response.Episodes))
			}
			for _, ep := range response.Episodes {
				want := tc.status
				if tc.missing || tc.status == model.DownloadStatusSuperseded || (tc.status == model.DownloadStatusCompleted && ep.EpisodeNumber > tc.archived) {
					want = "missing"
				}
				if ep.Status != want {
					t.Errorf("episode %d status=%s, want %s", ep.EpisodeNumber, ep.Status, want)
				}
				if want != "missing" && (ep.DownloadID != pack.ID || ep.Source != "mikan") {
					t.Errorf("episode %d lost pack association: %+v", ep.EpisodeNumber, ep)
				}
				if want == model.DownloadStatusCompleted && ep.FilePath == "" {
					t.Errorf("episode %d lost media path", ep.EpisodeNumber)
				}
			}
		})
	}
}

func TestEpisodeStatusArchiveWinsOverActivePack(t *testing.T) {
	db := testutil.InitTestDB()
	count, start, end, ep := 4, 1, 4, 2
	anime := model.Anime{Title: "archive priority", EpisodeCount: &count}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	pack := model.Download{TorrentID: "active-pack", Name: "pack", URL: "magnet", AnimeID: &anime.ID,
		Scope: model.DownloadScopeSeason, EpisodeStart: &start, EpisodeEnd: &end, Status: model.DownloadStatusDownloading}
	missing := model.Download{TorrentID: "old-missing", Name: "missing", URL: "magnet", AnimeID: &anime.ID,
		EpisodeNumber: &ep, Status: model.DownloadStatusCompleted, MediaMissing: true}
	if err := db.Create(&[]model.Download{pack, missing}).Error; err != nil {
		t.Fatal(err)
	}
	// Disk-adopted episodes need no download task; historical failures must
	// not hide them. The active pack deliberately has no trigger episode.
	path := "/media/archive S01E02.mkv"
	if err := db.Create(&model.AnimeEpisode{AnimeID: anime.ID, EpisodeNumber: ep, Downloaded: true, FilePath: &path}).Error; err != nil {
		t.Fatal(err)
	}
	router := testutil.SetupRouter()
	NewOrchestratorHandler(db, nil, nil).RegisterRoutes(router.Group("/api/v1"))
	w := testutil.MakeRequest(router, http.MethodGet, fmt.Sprintf("/api/v1/anime/%d/episode-status", anime.ID), nil, "")
	var response struct {
		Episodes []struct {
			Status     string `json:"status"`
			FilePath   string `json:"file_path"`
			DownloadID uint   `json:"download_id"`
		} `json:"episodes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || len(response.Episodes) != count {
		t.Fatalf("invalid response: %s", w.Body.String())
	}
	if got := response.Episodes[1]; got.Status != model.DownloadStatusCompleted || got.FilePath != path || got.DownloadID != 0 {
		t.Fatalf("disk archive lost: %+v", got)
	}
	if got := response.Episodes[2]; got.Status != model.DownloadStatusDownloading {
		t.Fatalf("pack without trigger lost: %+v", got)
	}
}
