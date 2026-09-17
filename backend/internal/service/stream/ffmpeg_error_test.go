package stream

import (
	"errors"
	"strings"
	"testing"
)

func TestFFmpegFailurePreservesCauseWithoutCredentials(t *testing.T) {
	err := errors.New("exit status 8")
	got := ffmpegFailure(err, []string{
		"out_time_ms=123", "progress=end",
		"Error opening https://user:secret@cdn.test/video?token=secret: HTTP error 403 Forbidden",
		"Error: Authorization: Bearer sensitive", "Failed request Cookie: session=secret",
	}).Error()
	for _, want := range []string{"exit status 8", "403 Forbidden", "[URL]", "[REDACTED]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q: %s", want, got)
		}
	}
	for _, secret := range []string{"secret", "sensitive", "cdn.test", "out_time_ms"} {
		if strings.Contains(got, secret) {
			t.Fatalf("unsafe/noisy diagnostic: %s", got)
		}
	}
	if !errors.Is(ffmpegFailure(err, nil), err) {
		t.Fatal("lost original exit error")
	}
}
