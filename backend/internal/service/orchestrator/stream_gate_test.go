package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestBrokenRuleClaimSurvivesIndependentOrchestrators(t *testing.T) {
	db := testutil.InitTestDB()
	broken, old := "broken", time.Now().Add(-2*time.Hour)
	rule := model.StreamRule{Name: "broken", HealthStatus: &broken, HealthAt: &old}
	db.Create(&rule)
	o := &Orchestrator{db: db}
	if !o.claimStreamRule(context.Background(), &rule) {
		t.Fatal("first half-open probe not admitted")
	}
	other := &Orchestrator{db: db}
	if other.claimStreamRule(context.Background(), &rule) {
		t.Fatal("stale in-memory health admitted a second probe")
	}
	var current model.StreamRule
	db.First(&current, rule.ID)
	if !isRuleBroken(&current) {
		t.Fatal("failed probe could immediately re-open")
	}
	db.Model(&rule).Update("health_at", old)
	db.Create(&model.Download{TorrentID: "still-probing", Name: "probe", URL: "probe", StreamRuleID: &rule.ID, Status: model.DownloadStatusDownloading})
	if o.claimStreamRule(context.Background(), &rule) {
		t.Fatal("unfinished probe did not hold the gate")
	}
}

func TestExhaustedCandidateBackoffAndIndependentRoadBudget(t *testing.T) {
	db := testutil.InitTestDB()
	o := &Orchestrator{db: db}
	aid, ep, rid := uint(7), 12, uint(1)
	detail, road := "catalog", "failed"
	for i := 0; i < 3; i++ {
		db.Create(&model.Download{TorrentID: fmt.Sprint(i), Name: "failed", URL: "signed-url", AnimeID: &aid, EpisodeNumber: &ep, StreamRuleID: &rid, StreamDetailURL: &detail, StreamRoadName: &road, DownloadType: model.DownloadTypeStream, Status: model.DownloadStatusFailed, FailureKind: model.FailureKindExhausted, RetryCount: 3, UpdatedAt: time.Now().Add(-25 * time.Hour)})
	}
	blocked, err := o.coolingStreamRoads(context.Background(), aid, ep, rid, detail, time.Now())
	if err != nil || !blocked[road] {
		t.Fatalf("repeat exhausted probe was reopened after one day: %v %v", blocked, err)
	}
	choice := &streamChoice{rule: &model.StreamRule{ID: rid}, detailURL: detail, episode: stream.EpisodeInfo{RoadName: "fresh"}}
	if got, err := o.streamCandidateRetryCount(context.Background(), aid, ep, choice); err != nil || got != 0 {
		t.Fatal("untried candidate inherited exhaustion", got)
	}
}
