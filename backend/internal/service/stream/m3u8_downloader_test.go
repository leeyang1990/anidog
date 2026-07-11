package stream

import "testing"

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
