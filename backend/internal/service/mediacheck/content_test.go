package mediacheck

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestProofMatchingCannotGrantPositiveAdmission(t *testing.T) {
	for _, scenario := range []string{"known", "unknown", "changed", "timeout", "old inspector", "repeated frame"} {
		t.Run(scenario, func(t *testing.T) {
			e := SubtitleEvidence{InspectorVersion: SubtitleInspectorVersion, Language: "traditional"}
			if scenario == "unknown" {
				e.Language = ""
			}
			if scenario == "old inspector" {
				e.InspectorVersion = "v1"
			}
			for i := 0; i < 4; i++ {
				frame := []byte(fmt.Sprint(i))
				if scenario == "repeated frame" {
					frame = []byte("repeated")
				}
				e.Samples = append(e.Samples, SubtitleSample{At: float64(i), FrameHash: FrameDigest(frame), Caption: "這個國家的故事還沒有結束"})
			}
			calls := 0
			got, err := matchSubtitleProof(context.Background(), e, func(ctx context.Context, at float64) ([]byte, error) {
				calls++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 21*time.Second {
					t.Fatal("unbounded remote match")
				}
				if scenario == "timeout" {
					return nil, context.DeadlineExceeded
				}
				if scenario == "changed" {
					return []byte("different captions"), nil
				}
				if scenario == "repeated frame" {
					return []byte("repeated"), nil
				}
				return []byte(fmt.Sprint(int(at))), nil
			})
			want := scenario == "known" || scenario == "unknown"
			if got != want {
				t.Fatal(got, err)
			}
			if scenario == "known" && calls != 2 || scenario == "unknown" && calls != 4 {
				t.Fatal("insufficient independent frame proof", calls)
			}
			if scenario == "old inspector" && calls != 0 {
				t.Fatal("stale evidence reused")
			}
			if scenario == "timeout" && err == nil {
				t.Fatal("execution failure hidden")
			}
		})
	}
}

func TestSubtitleEvidenceIncludesBlankFramesWithoutInventingCaptions(t *testing.T) {
	e := detectHardSubtitles(context.Background(), 1500, func(context.Context, float64) ([]byte, error) { return []byte("different pixels"), nil }, func(context.Context, []byte) (string, error) { return "", nil })
	if e.Language != "" || e.Characters != 0 || len(e.Samples) != 10 || e.InspectorVersion != SubtitleInspectorVersion {
		t.Fatal(e)
	}
	for _, s := range e.Samples {
		if s.Caption != "" || s.FrameHash == "" {
			t.Fatal(s)
		}
	}
}
