package stream

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/service/failure"
)

func TestFinishedTransferSurvivesInspectionTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX tool fixture")
	}
	dir := t.TempDir()
	probe := "#!/bin/sh\nfor arg do last=\"$arg\"; done\ncase \"$last\" in http*) printf '1200\\n';; *) exec /bin/sleep 10;; esac\n"
	transfer := "#!/bin/sh\nfor arg do last=\"$arg\"; done\nprintf 'finished-transfer-fixture' > \"$last\"\n"
	for name, script := range map[string]string{"ffprobe": probe, "ffmpeg": transfer} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := NewM3U8Downloader(&config.Config{FFMPEGPath: filepath.Join(dir, "ffmpeg"), StreamMaxConcurrent: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	path := filepath.Join(dir, "retained.mp4")
	start := time.Now()
	retained, err := d.DownloadVideo(ctx, "test", &InterceptedVideo{URL: "https://fixture.invalid/media.mp4"}, path, nil)
	if err == nil || failure.Describe(err).Code != failure.InspectionFailed || retained != path {
		t.Fatalf("inspection misclassified: %s, %v", retained, err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("inspection ignored the parent deadline")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "finished-transfer-fixture" {
		t.Fatal("tool failure discarded completed payload", err)
	}
}
