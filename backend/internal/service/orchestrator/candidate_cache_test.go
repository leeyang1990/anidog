package orchestrator

import (
	"context"
	"github.com/anidog/anidog-go/internal/config"
	downloadservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/indexer"
)

type countingIndexer struct {
	calls atomic.Int32
}

func (i *countingIndexer) Name() string { return "counting" }

func (i *countingIndexer) Search(context.Context, string) ([]indexer.Candidate, error) {
	i.calls.Add(1)
	return []indexer.Candidate{{Title: "Example - 01", InfoHash: "abc"}}, nil
}

func TestCollectBTCandidatesCachesPerAnime(t *testing.T) {
	idx := &countingIndexer{}
	o := New(nil, nil, nil, nil, map[string]indexer.Indexer{"counting": idx}, "/downloads")
	anime := &model.Anime{ID: 42, Title: "Example"}

	first := o.collectBTCandidates(context.Background(), anime, []indexer.Indexer{idx}, false)
	second := o.collectBTCandidates(context.Background(), anime, []indexer.Indexer{idx}, false)

	if got := idx.calls.Load(); got != 1 {
		t.Fatalf("indexer calls = %d, want 1", got)
	}
	if len(first.Candidates) != 1 || len(second.Candidates) != 1 {
		t.Fatalf("candidate counts = %d, %d; want 1, 1", len(first.Candidates), len(second.Candidates))
	}

	// 返回副本，调用方修改切片本身不能污染缓存。
	second.Candidates[0].Title = "changed"
	third := o.collectBTCandidates(context.Background(), anime, []indexer.Indexer{idx}, false)
	if third.Candidates[0].Title != "Example - 01" {
		t.Fatalf("cached candidate was mutated: %q", third.Candidates[0].Title)
	}
}

func TestCollectBTCandidatesSeparatesAnime(t *testing.T) {
	idx := &countingIndexer{}
	o := New(nil, nil, nil, nil, map[string]indexer.Indexer{"counting": idx}, "/downloads")

	o.collectBTCandidates(context.Background(), &model.Anime{ID: 1, Title: "One"}, []indexer.Indexer{idx}, false)
	o.collectBTCandidates(context.Background(), &model.Anime{ID: 2, Title: "Two"}, []indexer.Indexer{idx}, false)

	if got := idx.calls.Load(); got != 2 {
		t.Fatalf("indexer calls = %d, want 2", got)
	}
}

type recoveringIndexer struct {
	calls int
	fail  bool
}

func (i *recoveringIndexer) Name() string { return "recovering" }
func (i *recoveringIndexer) Search(context.Context, string) ([]indexer.Candidate, error) {
	i.calls++
	if i.fail {
		return nil, context.DeadlineExceeded
	}
	return nil, nil
}

func TestCandidateCacheSeparatesUnavailableFromSuccessfulEmpty(t *testing.T) {
	idx := &recoveringIndexer{fail: true}
	o := New(nil, nil, nil, nil, nil, "/downloads")
	anime := &model.Anime{ID: 42, Title: "Example"}
	enabled := []indexer.Indexer{idx}
	got := o.collectBTCandidates(context.Background(), anime, enabled, false)
	key := o.candidateCacheKey(anime, enabled, false)
	if got.Status() != "unavailable" || got.Warning() == "" {
		t.Fatal(got)
	}
	if ttl := time.Until(o.candidates[key].expiresAt); ttl > time.Minute || ttl <= 0 {
		t.Fatal("failure cached as healthy absence", ttl)
	}
	// Multiple missing episodes share a bounded failure observation, not new requests.
	got = o.collectBTCandidates(context.Background(), anime, enabled, false)
	if idx.calls != 1 || got.Status() != "unavailable" {
		t.Fatal(idx.calls, got)
	}
	got.FailedSources[0] = "changed"
	if o.candidates[key].result.FailedSources[0] != "recovering" {
		t.Fatal("cache mutated")
	}
	idx.fail = false
	entry := o.candidates[key]
	entry.expiresAt = time.Now().Add(-time.Second)
	o.candidates[key] = entry
	got = o.collectBTCandidates(context.Background(), anime, enabled, false)
	if got.Status() != "complete" || got.Warning() != "" || idx.calls != 2 {
		t.Fatal("recovery remained unavailable", got)
	}
	if time.Until(o.candidates[key].expiresAt) < 19*time.Minute {
		t.Fatal("healthy empty result lost normal caching")
	}
}

func TestFailedMikanRSSDoesNotMasqueradeAsHealthyEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer server.Close()
	o := New(nil, nil, nil, nil, nil, "/downloads")
	o.mikanRSS.BaseURL = server.URL
	mid := 42
	got := o.collectBTCandidates(context.Background(), &model.Anime{ID: 42, Title: "Example", MikanBangumiID: &mid}, []indexer.Indexer{&recoveringIndexer{}}, true)
	if got.Status() != "partial" || got.SuccessfulSources != 1 || len(got.FailedSources) != 1 || got.FailedSources[0] != "mikan_rss" {
		t.Fatal(got)
	}
}

func TestBTFailureDiagnosisUsesSearchAvailability(t *testing.T) {
	db := testutil.InitTestDB()
	svc := downloadservice.NewService(db, &config.Config{}, nil)
	svc.RegisterExecutor(model.DownloadTypeTorrent, &raceQueueExecutor{})
	idx := &recoveringIndexer{fail: true}
	o := New(db, svc, nil, nil, map[string]indexer.Indexer{idx.Name(): idx}, t.TempDir())
	anime := &model.Anime{ID: 42, Title: "Example"}
	pref := Preference{BTEnabled: true, EnabledIndexers: []string{idx.Name()}, Quality: "1080p", Languages: []string{"simplified"}}
	ok, n, _, reason, _, _ := o.tryBT(context.Background(), anime, 5, pref, 0, SourceBT, true)
	if ok || n != 0 || !strings.Contains(reason, "索引请求失败") || strings.Contains(reason, "均无结果") {
		t.Fatal("execution failure became absence", ok, n, reason)
	}
	idx.fail = false
	key := o.candidateCacheKey(anime, []indexer.Indexer{idx}, false)
	entry := o.candidates[key]
	entry.expiresAt = time.Now().Add(-time.Second)
	o.candidates[key] = entry
	ok, n, _, reason, _, _ = o.tryBT(context.Background(), anime, 5, pref, 0, SourceBT, true)
	if ok || n != 0 || !strings.Contains(reason, "均无结果") || strings.Contains(reason, "请求失败") {
		t.Fatal("recovered empty response retained old failure", reason)
	}
	var tasks int64
	db.Model(&model.Download{}).Count(&tasks)
	if tasks != 0 {
		t.Fatal("discovery created an unqualified transfer")
	}
}
