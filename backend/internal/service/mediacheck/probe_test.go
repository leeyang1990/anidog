package mediacheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestUnavailableProbeIsRetryable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Probe(context.Background(), "video.mp4")
	if err == nil || !IsToolError(err) {
		t.Fatalf("missing tooling must not reject the candidate: %v", err)
	}
}

func TestTailRejectsTruncatedVideoWithReadableIndex(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	path := filepath.Join(t.TempDir(), "clip.mp4")
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=10", "-t", "30", "-c:v", "mpeg4", "-g", "10", "-movflags", "+faststart", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	m, err := Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckTail(context.Background(), path, m.Duration()); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if err := os.Truncate(path, info.Size()*85/100); err != nil {
		t.Fatal(err)
	}
	// The container index still advertises the original 30 seconds.
	m, err = Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Duration() < 29 {
		t.Fatalf("fixture lost its original index: %v", m.Duration())
	}
	if err := CheckTail(context.Background(), path, m.Duration()); err == nil {
		t.Fatal("truncated payload accepted because its index remained readable")
	}
}
