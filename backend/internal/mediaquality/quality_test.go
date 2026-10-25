package mediaquality

import "testing"

func TestQualityLabels(t *testing.T) {
	for _, tc := range []struct {
		label string
		want  int
	}{
		{"480p", 480}, {"576p", 576}, {"720p", 720}, {"900p", 900},
		{"1080p", 1080}, {"FHD", 1080}, {"1440p", 1440},
		{"2160p", 2160}, {"4K", 2160}, {"uhd", 2160},
		{"4320p", 4320}, {"8K", 4320},
		{"", 0}, {"unknown", 0}, {"1080", 0}, {"1080p-or-better", 0},
	} {
		t.Run(tc.label, func(t *testing.T) {
			if got := Height(tc.label); got != tc.want {
				t.Fatalf("Height(%q) = %d; want %d", tc.label, got, tc.want)
			}
		})
	}
}
