package indexer

import (
	"context"
	"testing"
)

func TestAggregatePreservesFailuresAndSuccessfulAbsence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sources   []Indexer
		successes int
		status    string
		failed    int
	}{
		{"empty success", []Indexer{&mockIndexer{name: "empty"}}, 1, "complete", 0},
		{"all failed", []Indexer{&mockIndexer{name: "broken", err: context.DeadlineExceeded}}, 0, "unavailable", 1},
		{"partial candidates", []Indexer{&mockIndexer{name: "ok", results: []Candidate{{Title: "Example - 05 [1080p]", InfoHash: "hash"}}}, &mockIndexer{name: "broken", err: context.DeadlineExceeded}}, 1, "partial", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Aggregate(context.Background(), tc.sources, "Example")
			if got.SuccessfulSources != tc.successes || got.Status() != tc.status || len(got.FailedSources) != tc.failed {
				t.Fatal(got)
			}
			if (got.Warning() != "") != (tc.failed > 0) {
				t.Fatal("availability warning lost", got)
			}
			if tc.status == "partial" && (len(got.Candidates) != 1 || got.Candidates[0].Parsed == nil) {
				t.Fatal("healthy source blocked", got)
			}
		})
	}
}
