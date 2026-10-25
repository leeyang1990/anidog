package failure

import (
	"context"
	"errors"
	"fmt"
	"github.com/anidog/anidog-go/internal/model"
	"testing"
)

func TestTypedFactsSurviveWrappingAndDiagnosticChanges(t *testing.T) {
	for _, tc := range []struct {
		code   Code
		kind   string
		outage bool
	}{
		{IdentityMismatch, model.FailureKindRejected, false},
		{PreferenceMismatch, model.FailureKindRejected, false},
		{InspectionFailed, model.FailureKindTransient, false},
		{ToolUnavailable, model.FailureKindPermanent, false},
		{StorageUnavailable, model.FailureKindPermanent, false},
		{StateUnavailable, model.FailureKindPermanent, false},
		{SourceUnavailable, model.FailureKindTransient, true},
		{PlayerUnavailable, model.FailureKindRejected, true},
	} {
		for _, message := range []string{"renamed diagnostic", "permission denied; 流媒体候选无效"} {
			cause := errors.New(message)
			err := fmt.Errorf("outer: %w", Wrap(Subtitles, tc.code, cause))
			f := Describe(err)
			if f.Code != tc.code || f.Kind != tc.kind || f.AvailabilityFailure != tc.outage || !errors.Is(err, cause) {
				t.Fatalf("%s: %+v", tc.code, f)
			}
			row := model.Download{FailureCode: string(f.Code), FailureStage: string(f.Stage), FailureKind: model.FailureKindExhausted, LastError: "different persisted text"}
			if got := FromRecord(row); got != f {
				t.Fatalf("persisted facts diverged: %+v vs %+v", got, f)
			}
		}
	}
}

func TestCancellationAndCompatibilityDoNotInventAnOutage(t *testing.T) {
	if f := Describe(Wrap(Player, SourceUnavailable, context.Canceled)); f.AvailabilityFailure || f.Code != Canceled {
		t.Fatal(f)
	}
	if f := FromRecord(model.Download{FailureKind: model.FailureKindExhausted, LastError: "字幕检测暂时不可用：超时"}); f.AvailabilityFailure || f.Code != InspectionFailed {
		t.Fatal(f)
	}
	if f := FromRecord(model.Download{FailureCode: "future_code", FailureKind: model.FailureKindTransient}); f.AvailabilityFailure {
		t.Fatal("unknown future reason counted as outage")
	}
	if f := FromRecord(model.Download{FailureKind: model.FailureKindTransient, LastError: "HTTP 403"}); !f.AvailabilityFailure {
		t.Fatal("legacy network sample was lost")
	}
}

func TestHistoricalRejectedKindCannotInventContentEvidence(t *testing.T) {
	for _, tc := range []struct {
		message string
		code    Code
		class   string
		outage  bool
	}{
		{"流媒体预检探测失败: 无法读取成片: exit status 1 Server returned 404 Not Found", SourceUnavailable, "error", true},
		{"流媒体预检探测失败: 无法读取成片: context deadline exceeded", SourceUnavailable, "error", true},
		{"流媒体预检探测失败: ffprobe tool error", InspectionFailed, "error", false},
		{"unclassified old diagnostic", "", "error", false},
		{"流媒体预检不符合要求：实际分辨率 1280x720 低于要求 1080p", InvalidMedia, "rejected", false},
		{"流媒体候选无效：季数不匹配", InvalidMedia, "rejected", false},
	} {
		t.Run(tc.message, func(t *testing.T) {
			row := model.Download{Status: model.DownloadStatusFailed, FailureKind: model.FailureKindRejected, LastError: tc.message}
			f := FromRecord(row)
			if f.Code != tc.code || f.AvailabilityFailure != tc.outage || ResultClass(row) != tc.class {
				t.Fatal(f, ResultClass(row))
			}
		})
	}
	row := model.Download{Status: model.DownloadStatusFailed, FailureCode: string(PreferenceMismatch), FailureKind: model.FailureKindRejected, LastError: "流媒体预检探测失败: Server returned 404 Not Found"}
	if FromRecord(row).Code != PreferenceMismatch || ResultClass(row) != "rejected" {
		t.Fatal("legacy diagnostic overrode explicit typed facts")
	}
}
