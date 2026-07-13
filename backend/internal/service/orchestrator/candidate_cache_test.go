package orchestrator

import (
	"context"
	"sync/atomic"
	"testing"

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

	first := o.collectBTCandidates(context.Background(), anime, []indexer.Indexer{idx})
	second := o.collectBTCandidates(context.Background(), anime, []indexer.Indexer{idx})

	if got := idx.calls.Load(); got != 1 {
		t.Fatalf("indexer calls = %d, want 1", got)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("candidate counts = %d, %d; want 1, 1", len(first), len(second))
	}

	// 返回副本，调用方修改切片本身不能污染缓存。
	second[0].Title = "changed"
	third := o.collectBTCandidates(context.Background(), anime, []indexer.Indexer{idx})
	if third[0].Title != "Example - 01" {
		t.Fatalf("cached candidate was mutated: %q", third[0].Title)
	}
}

func TestCollectBTCandidatesSeparatesAnime(t *testing.T) {
	idx := &countingIndexer{}
	o := New(nil, nil, nil, nil, map[string]indexer.Indexer{"counting": idx}, "/downloads")

	o.collectBTCandidates(context.Background(), &model.Anime{ID: 1, Title: "One"}, []indexer.Indexer{idx})
	o.collectBTCandidates(context.Background(), &model.Anime{ID: 2, Title: "Two"}, []indexer.Indexer{idx})

	if got := idx.calls.Load(); got != 2 {
		t.Fatalf("indexer calls = %d, want 2", got)
	}
}
