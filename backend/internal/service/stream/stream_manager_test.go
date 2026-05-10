package stream

import (
	"testing"
)

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"normal.mp4", "normal.mp4"},
		{"path/to/file.mp4", "path_to_file.mp4"},
		{"file:name?.mp4", "file_name_.mp4"},
		{"concurrent<>.mp4", "concurrent__.mp4"},
		{"quote\"file.mp4", "quote_file.mp4"},
		{"star*file.mp4", "star_file.mp4"},
		{"pipe|file.mp4", "pipe_file.mp4"},
	}
	for _, tt := range tests {
		got := sanitizeFileName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFileName(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}
