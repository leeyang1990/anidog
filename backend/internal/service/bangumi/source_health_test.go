package bangumi

import (
	"context"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestRuleHealthCountsOnlyCompletedTransfersAndAvailabilityFailures(t *testing.T) {
	db := testutil.InitTestDB()
	rule := model.StreamRule{Name: "rule"}
	db.Create(&rule)
	for _, status := range []string{model.DownloadStatusPending, model.DownloadStatusSuperseded, model.DownloadStatusFailed} {
		db.Create(&model.Download{TorrentID: status, Name: status, URL: status, StreamRuleID: &rule.ID, DownloadType: model.DownloadTypeStream, Status: status, FailureKind: model.FailureKindRejected})
	}
	s := NewSourceHealthService(db)
	s.updateRuleHealth(context.Background())
	db.First(&rule, rule.ID)
	if rule.HealthStatus != nil && *rule.HealthStatus != "" {
		t.Fatal("unfinished/quality-rejected tasks were counted as transfers", *rule.HealthStatus)
	}
	db.Create(&model.Download{TorrentID: "network", Name: "network", URL: "network", StreamRuleID: &rule.ID, DownloadType: model.DownloadTypeStream, Status: model.DownloadStatusFailed, FailureKind: model.FailureKindTransient})
	s.updateRuleHealth(context.Background())
	db.First(&rule, rule.ID)
	if rule.HealthStatus == nil || *rule.HealthStatus != "broken" || rule.HealthNote == nil || *rule.HealthNote != "24h 内：0 完成 / 1 失败" {
		t.Fatal(rule)
	}
}
