package mediacheck

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestPartialOCRFailureCannotEraseConflictingSubtitleFacts(t *testing.T) {
	call := 0
	e := detectHardSubtitles(context.Background(), 1500, func(_ context.Context, at float64) ([]byte, error) { return []byte(fmt.Sprint(at)), nil }, func(context.Context, []byte) (string, error) {
		call++
		switch call {
		case 1:
			return "這個國家的故事還沒有結束我們會再見面", nil
		case 2:
			return "这是我们的世界这个国家学校还有许多故事对话发展", nil
		case 3:
			return "", context.DeadlineExceeded
		default:
			return "", nil
		}
	})
	if e.Language != "" {
		t.Fatal("conflict granted script eligibility", e)
	}
	if e.Retryable {
		t.Fatal("one failed sample discarded immutable conflicting observations", e)
	}
	if e.OCRErrors != 1 || len(e.Samples) != 20 {
		t.Fatal("partial failure diagnostics were lost", e)
	}
}

func conflictFixture() SubtitleEvidence {
	return SubtitleEvidence{InspectorVersion: "subtitle-v3", Method: "hard_subtitles", Duration: 1500, Width: 1920, Height: 1080, Attempted: 3, Extracted: 3, Recognized: 2, OCRErrors: 1, Retryable: true, Samples: []SubtitleSample{
		{At: 100, FrameHash: fmt.Sprintf("%064x", 1), Caption: "這個國家的故事還沒有結束我們會再見面"},
		{At: 200, FrameHash: fmt.Sprintf("%064x", 2), Caption: "这是我们的世界这个国家学校还有许多故事对话发展"},
		{At: 300, FrameHash: fmt.Sprintf("%064x", 3), FailureStage: "ocr", FailureReason: "timeout"},
	}}
}

func TestRetainedAssessmentPreservesCompatibleObservationsWithoutAdmission(t *testing.T) {
	old := conflictFixture()
	got, ok := ReassessRetainedSubtitles(old)
	if !ok || !got.Conflicting || got.Retryable || got.Language != "" || got.InspectorVersion != SubtitleInspectorVersion || got.ObservationVersion != SubtitleObservationVersion || got.ReassessedFrom != "subtitle-v3" {
		t.Fatal("incorrect reassessment", got, ok)
	}
	if !reflect.DeepEqual(got.Samples, old.Samples) || got.OCRErrors != 1 || old.InspectorVersion != "subtitle-v3" {
		t.Fatal("measurement provenance was rewritten")
	}
	for _, mutate := range []func(*SubtitleEvidence){
		func(e *SubtitleEvidence) { e.InspectorVersion = "unrecognized" },
		func(e *SubtitleEvidence) { e.ObservationVersion = "different-models" },
		func(e *SubtitleEvidence) { e.Language = "simplified" },
		func(e *SubtitleEvidence) { e.Method = "subtitle_tracks" },
		func(e *SubtitleEvidence) { e.Recognized++ },
		func(e *SubtitleEvidence) { e.Duration = 50 },
		func(e *SubtitleEvidence) { e.Samples[0].FrameHash = "unverified" },
		func(e *SubtitleEvidence) { e.Samples[2].FailureReason = "canceled" },
	} {
		invalid := old
		invalid.Samples = append([]SubtitleSample(nil), old.Samples...)
		mutate(&invalid)
		if _, ok := ReassessRetainedSubtitles(invalid); ok {
			t.Fatal("incompatible or incomplete provenance reused", invalid)
		}
	}
	onlyOutage := old
	onlyOutage.Samples = append([]SubtitleSample(nil), old.Samples...)
	for i := range onlyOutage.Samples {
		onlyOutage.Samples[i].Caption = ""
		onlyOutage.Samples[i].FailureStage = "ocr"
		onlyOutage.Samples[i].FailureReason = "timeout"
	}
	onlyOutage.Recognized, onlyOutage.OCRErrors = 0, 3
	if _, ok := ReassessRetainedSubtitles(onlyOutage); ok {
		t.Fatal("outage was converted into content knowledge")
	}
}
