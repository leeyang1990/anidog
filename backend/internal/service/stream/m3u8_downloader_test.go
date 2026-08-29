package stream

import (
	"strings"
	"testing"
)

func TestParseFFmpegProgressSizeDoesNotResetProgress(t *testing.T) {
	d := &M3U8Downloader{}

	progress, downloadedBytes, ok := d.parseFFmpegProgress("total_size=1048576", 120)
	if !ok {
		t.Fatal("expected total_size line to be parsed")
	}
	if progress != -1 {
		t.Fatalf("expected unknown progress sentinel -1, got %v", progress)
	}
	if downloadedBytes != 1048576 {
		t.Fatalf("expected downloaded bytes 1048576, got %d", downloadedBytes)
	}
}

func TestParseFFmpegProgressTime(t *testing.T) {
	d := &M3U8Downloader{}

	progress, downloadedBytes, ok := d.parseFFmpegProgress("out_time_ms=60000000", 120)
	if !ok {
		t.Fatal("expected out_time_ms line to be parsed")
	}
	if progress != 50 {
		t.Fatalf("expected progress 50, got %v", progress)
	}
	if downloadedBytes != 0 {
		t.Fatalf("expected no downloaded byte update, got %d", downloadedBytes)
	}
}

func TestValidateVideoProbeRejectsShortEpisode(t *testing.T) {
	probe := videoProbeResult{CodecName: "h264", Duration: 3.12, Size: 784108}
	err := validateVideoProbe(probe, 0, 300)
	if err == nil || !strings.Contains(err.Error(), "视频时长过短") {
		t.Fatalf("got %v, want short-duration rejection", err)
	}
}

func TestValidateVideoProbeRejectsTruncatedDownload(t *testing.T) {
	probe := videoProbeResult{CodecName: "hevc", Duration: 600, Size: 200 * 1024 * 1024}
	err := validateVideoProbe(probe, 1480, 300)
	if err == nil || !strings.Contains(err.Error(), "视频疑似下载截断") {
		t.Fatalf("got %v, want truncation rejection", err)
	}
}

func TestValidateVideoProbeAcceptsNormalEpisode(t *testing.T) {
	probe := videoProbeResult{CodecName: "h264", Duration: 1480, Size: 790128053}
	if err := validateVideoProbe(probe, 1480, 300); err != nil {
		t.Fatalf("normal episode rejected: %v", err)
	}
}

func TestValidateVideoProbeAllowsConfiguredShortAnime(t *testing.T) {
	probe := videoProbeResult{CodecName: "h264", Duration: 199.616, Size: 115900071}
	if err := validateVideoProbe(probe, 0, 0); err != nil {
		t.Fatalf("disabled minimum duration rejected short anime: %v", err)
	}
}
