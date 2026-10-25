package mediacheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// VideoDigest hashes the complete compressed video, not container metadata or
// filenames. Subtitle tracks are deliberately evaluated separately per asset.
func VideoDigest(ctx context.Context, path, codec string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	args := []string{"-nostdin", "-v", "error", "-i", path, "-map", "0:v:0", "-c:v", "copy", "-an", "-sn"}
	switch codec {
	case "h264":
		args = append(args, "-bsf:v", "h264_mp4toannexb")
	case "hevc":
		args = append(args, "-bsf:v", "hevc_mp4toannexb")
	}
	args = append(args, "-f", "hash", "-hash", "sha256", "pipe:1")
	out, err := MediaCommand(ctx, "ffmpeg", args...).Output()
	if err != nil {
		return "", toolError{fmt.Errorf("视频内容指纹检查失败: %w", commandError(ctx, err))}
	}
	digest := strings.TrimSpace(strings.TrimPrefix(string(out), "SHA256="))
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size {
		return "", toolError{fmt.Errorf("视频内容指纹输出无效")}
	}
	return "video-v1:" + codec + ":" + strings.ToLower(digest), nil
}

func FrameDigest(frame []byte) string { sum := sha256.Sum256(frame); return hex.EncodeToString(sum[:]) }

// MatchSubtitleProof checks exact images of independently confirmed captions.
// It is a bounded negative preflight, never proof of whole-file identity or a
// substitute for local integrity/quality acceptance. An inconclusive inspection
// requires four distinct matching frames and stays inconclusive when reused.
func MatchSubtitleProof(ctx context.Context, input string, args []string, evidence SubtitleEvidence) (bool, error) {
	return matchSubtitleProof(ctx, evidence, func(ctx context.Context, at float64) ([]byte, error) {
		return ExtractSubtitleFrame(ctx, input, args, at)
	})
}

func matchSubtitleProof(ctx context.Context, evidence SubtitleEvidence, extract func(context.Context, float64) ([]byte, error)) (bool, error) {
	if evidence.InspectorVersion != SubtitleInspectorVersion || evidence.Retryable {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	matched := 0
	seen := map[string]bool{}
	required := 2
	if evidence.Language == "" {
		required = 4
	}
	for _, sample := range evidence.Samples {
		if sample.FrameHash == "" || seen[sample.FrameHash] {
			continue
		}
		var votes scriptVotes
		votes.addCaption(sample.Caption)
		if evidence.Language == "traditional" && votes.traditionalFrames == 0 || evidence.Language == "simplified" && votes.simpleFrames == 0 {
			continue
		}
		frameCtx, stop := context.WithTimeout(ctx, 20*time.Second)
		frame, err := extract(frameCtx, sample.At)
		stop()
		if err != nil {
			return false, err
		}
		if FrameDigest(frame) != sample.FrameHash {
			return false, nil
		}
		seen[sample.FrameHash] = true
		matched++
		if matched >= required {
			return true, nil
		}
	}
	return false, nil
}
