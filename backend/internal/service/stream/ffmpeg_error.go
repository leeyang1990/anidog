package stream

import (
	"fmt"
	"regexp"
	"strings"
)

var diagnosticURL = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
var diagnosticCredential = regexp.MustCompile(`(?i)(authorization|cookie|set-cookie|proxy-authorization)\s*:[^\r\n]*`)

// ffmpegFailure persists a small, safe diagnostic with the task, rather than
// leaving the only actionable cause in an ephemeral container's stderr.
func ffmpegFailure(err error, lines []string) error {
	var useful []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "error") && !strings.Contains(lower, "failed") &&
			!strings.Contains(lower, "invalid") && !strings.Contains(lower, "denied") &&
			!strings.Contains(lower, "no space") && !strings.Contains(lower, "read-only") &&
			!strings.Contains(lower, "timed out") && !strings.Contains(lower, "forbidden") {
			continue
		}
		line = diagnosticCredential.ReplaceAllString(line, "$1: [REDACTED]")
		line = diagnosticURL.ReplaceAllString(line, "[URL]")
		if chars := []rune(line); len(chars) > 256 {
			line = string(chars[:256]) + "…"
		}
		useful = append(useful, line)
	}
	if len(useful) == 0 {
		return fmt.Errorf("ffmpeg 退出: %w", err)
	}
	if len(useful) > 4 {
		useful = useful[len(useful)-4:]
	}
	return fmt.Errorf("ffmpeg 退出: %w; %s", err, strings.Join(useful, "; "))
}
