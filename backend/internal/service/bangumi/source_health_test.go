package bangumi

import (
	"context"
	"fmt"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
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

func TestHealthUsesSameFailureFactsForRuleAndAnime(t *testing.T) {
	db := testutil.InitTestDB()
	rule := model.StreamRule{Name: "health-facts"}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	detail, road := "detail", "road"
	anime := model.Anime{Title: "health-facts", IsSubscribed: true, StreamRuleID: &rule.ID, StreamDetailURL: &detail, StreamRoadName: &road}
	if err := db.Create(&anime).Error; err != nil {
		t.Fatal(err)
	}
	for i, code := range []failure.Code{failure.InspectionFailed, failure.Canceled, failure.ToolUnavailable, failure.StateUnavailable, failure.PreferenceMismatch, ""} {
		row := model.Download{TorrentID: fmt.Sprint("ignored-", i), Name: "diagnostic", URL: "play", AnimeID: &anime.ID, StreamRuleID: &rule.ID, StreamDetailURL: &detail, StreamRoadName: &road, DownloadType: model.DownloadTypeStream, Status: model.DownloadStatusFailed, FailureKind: model.FailureKindExhausted, FailureCode: string(code), FailureStage: string(failure.Subtitles), LastError: "字幕检测暂时不可用：timeout"}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := NewSourceHealthService(db)
	svc.CheckAllSubscribed(context.Background())
	if err := db.First(&rule, rule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&anime, anime.ID).Error; err != nil {
		t.Fatal(err)
	}
	if (rule.HealthStatus != nil && *rule.HealthStatus != "") || (anime.SourceHealthStatus != nil && *anime.SourceHealthStatus != "") {
		t.Fatal("local inspection failures became a source outage")
	}
	// A real network failure must still contribute; translated text is irrelevant.
	row := model.Download{TorrentID: "outage", Name: "outage", URL: "play", AnimeID: &anime.ID, StreamRuleID: &rule.ID, StreamDetailURL: &detail, StreamRoadName: &road, DownloadType: model.DownloadTypeStream, Status: model.DownloadStatusFailed, FailureKind: model.FailureKindTransient, FailureCode: string(failure.SourceUnavailable), LastError: "new message"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	svc.CheckAllSubscribed(context.Background())
	if err := db.First(&rule, rule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&anime, anime.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rule.HealthStatus == nil || *rule.HealthStatus != SourceBroken || anime.SourceHealthStatus == nil || *anime.SourceHealthStatus != SourceBroken {
		t.Fatal("real outages stopped contributing")
	}
	// Repeated health aggregation must not extend the circuit lease.
	before := *rule.HealthAt
	svc.CheckAllSubscribed(context.Background())
	db.First(&rule, rule.ID)
	if rule.HealthAt == nil || !rule.HealthAt.Equal(before) {
		t.Fatal("unchanged circuit lease was extended")
	}
}
