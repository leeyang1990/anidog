package download

import (
	"strings"
	"testing"
)

func TestValidateDurationAgainstPeersRejectsShortPreview(t *testing.T) {
	err := validateDurationAgainstPeers(199.616, []float64{1430.08, 1475.099, 1538.57})
	if err == nil || !strings.Contains(err.Error(), "同番中位数") {
		t.Fatalf("got %v, want short-preview rejection", err)
	}
}

func TestValidateDurationAgainstPeersAllowsShortFormEpisode(t *testing.T) {
	if err := validateDurationAgainstPeers(100, []float64{180, 190, 200}); err != nil {
		t.Fatalf("short-form episode rejected: %v", err)
	}
}

func TestValidateDurationAgainstPeersRequiresTwoSamples(t *testing.T) {
	if err := validateDurationAgainstPeers(100, []float64{1500}); err != nil {
		t.Fatalf("single peer must not establish a baseline: %v", err)
	}
}

func TestValidateDurationAgainstPeersUsesMedian(t *testing.T) {
	if err := validateDurationAgainstPeers(705, []float64{1400, 1420, 5000}); err != nil {
		t.Fatalf("candidate at 50%% of median rejected: %v", err)
	}
}
