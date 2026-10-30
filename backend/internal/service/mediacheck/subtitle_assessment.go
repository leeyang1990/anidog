package mediacheck

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
)

// Measurement and assessment versions are separate. v4 changes the treatment
// of contradictory observations, while extraction, models and confidence rules
// remain the v3 measurement contract. Changing those requires a new observation
// version; unknown versions must be measured again.
const SubtitleObservationVersion = "caption-v3"

func assessSubtitleObservations(e SubtitleEvidence, votes scriptVotes, executionErr error) SubtitleEvidence {
	result := votes.result()
	e.Language, e.Frames, e.Characters, e.Reason = result.Language, result.Frames, result.Characters, result.Reason
	e.SimplifiedGlyphs, e.TraditionalGlyphs = result.SimplifiedGlyphs, result.TraditionalGlyphs
	e.Conflicting = e.Language == "" && votes.chinese >= 20 && votes.simpleFrames > 0 && votes.traditionalFrames > 0
	e.Retryable = false
	if e.Conflicting {
		// Additional successful reads cannot remove observations already obtained.
		// Keep the conflict unknown; it never grants either script's eligibility.
		e.Reason = fmt.Sprintf("字幕识别读数存在简繁冲突，无法可靠确认字幕类型（有效文字 %d 字，简体区分字 %d 种、繁体区分字 %d 种）", e.Characters, e.SimplifiedGlyphs, e.TraditionalGlyphs)
	}
	if e.Language == "" && (errors.Is(executionErr, context.Canceled) || !e.Conflicting && (e.FrameErrors > 0 || e.OCRErrors > 0 || executionErr != nil)) {
		e.Retryable = true
		e.Reason = fmt.Sprintf("字幕检测执行失败：抽帧 %d/%d 成功（%d 失败），识别 %d 成功（%d 失败）", e.Extracted, e.Attempted, e.FrameErrors, e.Recognized, e.OCRErrors)
	}
	return e
}

// ReassessRetainedSubtitles recovers a stable conflict from compatible stored
// observations. The caller must first verify the complete video digest and
// geometry/duration. This only recovers an unknown result, never admission or
// a new script claim. Original hashes, captions and failed-sample diagnostics
// are preserved with their measurement version and producer.
func ReassessRetainedSubtitles(previous SubtitleEvidence) (SubtitleEvidence, bool) {
	if previous.Language != "" || previous.Method != "hard_subtitles" || (previous.InspectorVersion != "subtitle-v3" && previous.InspectorVersion != SubtitleInspectorVersion) {
		return SubtitleEvidence{}, false
	}
	observation := previous.ObservationVersion
	if observation == "" && previous.InspectorVersion == "subtitle-v3" {
		observation = "caption-v3"
	}
	if observation != SubtitleObservationVersion || previous.Duration <= 0 || math.IsInf(previous.Duration, 0) || math.IsNaN(previous.Duration) || len(previous.Samples) < 2 || len(previous.Samples) > 20 || previous.Attempted != len(previous.Samples) {
		return SubtitleEvidence{}, false
	}
	var votes scriptVotes
	extracted, recognized, frameErrors, ocrErrors := 0, 0, 0, 0
	for _, sample := range previous.Samples {
		if sample.At <= 0 || sample.At >= previous.Duration || math.IsNaN(sample.At) || sample.FailureReason == "canceled" {
			return SubtitleEvidence{}, false
		}
		if sample.FailureStage == "frame" {
			frameErrors++
			continue
		}
		hash, err := hex.DecodeString(sample.FrameHash)
		if err != nil || len(hash) != 32 {
			return SubtitleEvidence{}, false
		}
		extracted++
		switch sample.FailureStage {
		case "ocr":
			ocrErrors++
		case "":
			recognized++
			votes.addCaption(sample.Caption)
		default:
			return SubtitleEvidence{}, false
		}
	}
	if previous.Extracted != extracted || previous.Recognized != recognized || previous.FrameErrors != frameErrors || previous.OCRErrors != ocrErrors {
		return SubtitleEvidence{}, false
	}
	e := assessSubtitleObservations(previous, votes, nil)
	if !e.Conflicting || e.Language != "" || e.Retryable {
		return SubtitleEvidence{}, false
	}
	e.InspectorVersion, e.ObservationVersion = SubtitleInspectorVersion, observation
	if previous.InspectorVersion != SubtitleInspectorVersion {
		e.ReassessedFrom = previous.InspectorVersion
	}
	return e, true
}
