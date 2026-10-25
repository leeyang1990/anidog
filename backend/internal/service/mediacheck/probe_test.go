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

func TestProbeKeepsActualMP4SubtitleHandlerLabel(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}
	dir := t.TempDir()
	subtitle := filepath.Join(dir, "simple.srt")
	path := filepath.Join(dir, "native.mp4")
	if err := os.WriteFile(subtitle, []byte("1\n00:00:00,000 --> 00:00:01,800\n这是简体字幕\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=s=160x90:r=2:d=2", "-i", subtitle, "-map", "0:v", "-map", "1:s", "-c:v", "mpeg4", "-c:s", "mov_text", "-metadata:s:s:0", "language=chi", "-metadata:s:s:0", "handler_name=中文（简体）", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	media, err := Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stream := range media.Streams {
		if stream.CodecType == "subtitle" {
			if stream.Tags["handler_name"] != "中文（简体）" {
				t.Fatal("probe discarded the container's subtitle label", stream.Tags)
			}
			languages := SubtitleLanguages(stream.Tags)
			if len(languages) != 1 || languages[0] != "simplified" {
				t.Fatal(languages)
			}
			return
		}
	}
	t.Fatal("native mov_text track missing")
}
