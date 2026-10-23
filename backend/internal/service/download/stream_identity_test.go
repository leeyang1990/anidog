package download

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
)

type identityCatalog struct {
	results  []stream.SearchResult
	episodes []stream.EpisodeInfo
	err      error
	deadline bool
}

func (c *identityCatalog) SearchAnime(ctx context.Context, _ *model.StreamRule, _ string) ([]stream.SearchResult, error) {
	_, c.deadline = ctx.Deadline()
	return c.results, c.err
}

func (c *identityCatalog) GetEpisodes(context.Context, *model.StreamRule, string) ([]stream.EpisodeInfo, error) {
	return c.episodes, c.err
}

func TestStreamExecutionRevalidatesPersistedSourceIdentity(t *testing.T) {
	for _, name := range []string{"correct", "wrong arc", "missing arc", "missing result", "changed URL", "wrong number", "wrong road", "missing detail", "network failure", "named episodes", "combined season"} {
		t.Run(name, func(t *testing.T) {
			aid, ep := uint(1), 1
			count := 8
			anime := &model.Anime{Title: "Re：从零开始的异世界生活 第四季 夺还篇", EpisodeCount: &count}
			task := &Task{Source: SourceStream, SourceTitle: anime.Title, AnimeID: &aid, EpisodeNumber: &ep, StreamRule: &model.StreamRule{}, StreamDetailURL: "detail", StreamRoadName: "main", URL: "play"}
			catalog := &identityCatalog{results: []stream.SearchResult{{Name: anime.Title, URL: "detail"}}, episodes: []stream.EpisodeInfo{{Name: "1", URL: "play", RoadName: "main"}}}
			switch name {
			case "wrong arc":
				catalog.results[0].Name = "Re：从零开始的异世界生活 第四季 丧失篇"
			case "missing arc":
				catalog.results[0].Name = "Re：从零开始的异世界生活 第四季"
			case "missing result":
				catalog.results = nil
			case "changed URL":
				catalog.episodes[0].URL = "other"
			case "wrong number":
				catalog.episodes[0].Name = "第12集"
			case "wrong road":
				catalog.episodes[0].RoadName = "other"
			case "missing detail":
				task.StreamDetailURL = ""
			case "network failure":
				catalog.err = errors.New("catalog timeout")
			case "named episodes":
				catalog.episodes[0].Name = "これからの話"
			case "combined season":
				catalog.episodes = append(catalog.episodes, stream.EpisodeInfo{Name: "19", URL: "combined", RoadName: "main"})
			}
			e := &StreamExecutor{catalog: catalog}
			err := e.validateSource(context.Background(), task, anime)
			if name == "correct" || name == "named episodes" {
				if err != nil || !catalog.deadline {
					t.Fatalf("valid mapping failed or unbounded request: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("stale persisted title bypassed the live catalog")
			}
			kind, _ := classifyError(err, 0)
			want := model.FailureKindRejected
			if name == "network failure" || name == "missing result" {
				want = model.FailureKindTransient
			}
			if kind != want {
				t.Fatalf("%v classified as %s, want %s", err, kind, want)
			}
		})
	}
}

func TestExplicitStreamRetryCannotReuseWrongArc(t *testing.T) {
	s := capacityService(t, 500*gib)
	a := model.Anime{Title: "Re：从零开始的异世界生活 第四季 夺还篇"}
	if err := s.db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.AutoMigrate(&model.StreamRule{}); err != nil {
		t.Fatal(err)
	}
	rule := model.StreamRule{Name: "catalog"}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	ep, detail, road := 1, "detail", "main"
	row := model.Download{TorrentID: "stream_old", Name: a.Title, SourceTitle: a.Title, URL: "play", AnimeID: &a.ID, EpisodeNumber: &ep, StreamRuleID: &rule.ID, StreamDetailURL: &detail, StreamRoadName: &road, DownloadType: model.DownloadTypeStream, Source: SourceStream, Status: model.DownloadStatusFailed}
	if err := s.db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	// No media manager: any attempt to intercept or write media would panic.
	e := &StreamExecutor{db: s.db, catalog: &identityCatalog{results: []stream.SearchResult{{Name: "Re：从零开始的异世界生活 第四季", URL: detail}}}}
	s.RegisterExecutor(model.DownloadTypeStream, e)
	if _, err := s.Retry(context.Background(), row.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var saved model.Download
		if err := s.db.First(&saved, row.ID).Error; err != nil {
			t.Fatal(err)
		}
		if saved.Status == model.DownloadStatusFailed {
			if saved.FailureKind != model.FailureKindRejected || !strings.Contains(saved.LastError, "篇章") || saved.NextRetryAt != nil {
				t.Fatal(saved)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("wrong-arc retry did not reach a safe terminal state")
}

func TestStreamExecutionDoesNotIgnoreMissingAnime(t *testing.T) {
	s := capacityService(t, 500*gib)
	aid := uint(999)
	e := &StreamExecutor{db: s.db}
	_, err := e.Execute(context.Background(), &Task{AnimeID: &aid, StreamRule: &model.StreamRule{}, Source: SourceStream}, nil)
	if err == nil || !strings.Contains(err.Error(), "读取流媒体目标番剧失败") {
		t.Fatalf("missing target fell back to a legacy file path: %v", err)
	}
}

type cancelledIdentityCatalog struct{ entered chan struct{} }

func (c *cancelledIdentityCatalog) SearchAnime(ctx context.Context, _ *model.StreamRule, _ string) ([]stream.SearchResult, error) {
	close(c.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (*cancelledIdentityCatalog) GetEpisodes(context.Context, *model.StreamRule, string) ([]stream.EpisodeInfo, error) {
	panic("cancelled source validation reached the playlist")
}

func TestStreamCancellationIncludesIdentityValidation(t *testing.T) {
	s := capacityService(t, 500*gib)
	a := model.Anime{Title: "Target"}
	if err := s.db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	catalog := &cancelledIdentityCatalog{entered: make(chan struct{})}
	e := &StreamExecutor{db: s.db, catalog: catalog}
	ep := 1
	done := make(chan error, 1)
	go func() {
		_, err := e.Execute(context.Background(), &Task{RuntimeID: "cancel-source", AnimeID: &a.ID, EpisodeNumber: &ep, StreamDetailURL: "detail", StreamRule: &model.StreamRule{}, Source: SourceStream}, nil)
		done <- err
	}()
	select {
	case <-catalog.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("source validation did not start")
	}
	if err := e.Cancel("cancel-source"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("validation ignored cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled source kept running")
	}
}
