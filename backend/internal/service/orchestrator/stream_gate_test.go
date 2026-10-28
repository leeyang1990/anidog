package orchestrator

import (
	"context"
	"fmt"
	"github.com/anidog/anidog-go/internal/config"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"os"
	"path/filepath"
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

func TestRetainedStreamAssetOutlivesSourceCooldown(t *testing.T) {
	for _, kind := range []string{model.FailureKindReview, model.FailureKindExhausted} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.InitTestDB()
			root := t.TempDir()
			svc := dlservice.NewService(db, &config.Config{MediaRoot: root}, nil)
			o := &Orchestrator{db: db, dlSvc: svc, streamMgr: &fakeStreamCatalog{}}
			rule := model.StreamRule{Name: "catalog", Enabled: true, MultiSources: true}
			db.Create(&rule)
			aid, ep := uint(7), 1
			detail, road := fmt.Sprintf("detail-%d", rule.ID), "A"
			path, final := filepath.Join(root, "payload.part.mp4"), filepath.Join(root, "payload.mp4")
			if err := os.WriteFile(path, []byte("retained bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			row := model.Download{TorrentID: "retained", Name: "asset", URL: "old-signature", AnimeID: &aid, EpisodeNumber: &ep, StreamRuleID: &rule.ID, StreamDetailURL: &detail, StreamRoadName: &road, DownloadType: model.DownloadTypeStream, Status: model.DownloadStatusFailed, FailureKind: kind, ValidationPending: true, FilePath: &path, ValidationFinalPath: final, UpdatedAt: time.Now().Add(-8 * 24 * time.Hour)}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			// A newer failed network probe cannot hide an older retained asset.
			newer := row
			newer.ID = 0
			newer.TorrentID = "network-probe"
			newer.ValidationPending = false
			newer.FilePath = nil
			newer.ValidationFinalPath = ""
			newer.FailureKind = model.FailureKindTransient
			if err := db.Create(&newer).Error; err != nil {
				t.Fatal(err)
			}
			anime := model.Anime{ID: aid, Title: "example", StreamRuleID: &rule.ID, StreamDetailURL: &detail, StreamRoadName: &road}
			choice, err := o.resolveStreamSource(context.Background(), &anime, ep)
			if err != nil || choice.episode.RoadName != "B" {
				t.Fatal("re-downloaded retained source rather than selecting another road", choice, err)
			}
			// Restarted planners consult actual asset availability, not a process cache.
			restarted := &Orchestrator{db: db, dlSvc: dlservice.NewService(db, &config.Config{MediaRoot: root}, nil)}
			blocked, err := restarted.coolingStreamRoads(context.Background(), aid, ep, rule.ID, detail, time.Now())
			if err != nil || !blocked[road] {
				t.Fatal("retained owner lost after restart", blocked, err)
			}
			if err = os.Remove(path); err != nil {
				t.Fatal(err)
			}
			blocked, err = restarted.coolingStreamRoads(context.Background(), aid, ep, rule.ID, detail, time.Now())
			if err != nil || blocked[road] {
				t.Fatal("missing retained bytes prevented recovery transfer", blocked, err)
			}
			outside := filepath.Join(t.TempDir(), "outside")
			if err = os.WriteFile(outside, []byte("bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			if _, err = restarted.coolingStreamRoads(context.Background(), aid, ep, rule.ID, detail, time.Now()); err == nil {
				t.Fatal("unsafe retained path silently permitted a new transfer")
			}
		})
	}
}
