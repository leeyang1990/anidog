package desktopappearance

import (
	"runtime"
	"testing"
)

func TestStateFromFlags(t *testing.T) {
	for _, tc := range []struct {
		flags                int
		mode                 string
		transparency, motion bool
	}{
		{0, "none", false, false}, {1, "liquid", false, false},
		{2, "vibrancy", false, false}, {7, "solid", true, false},
		{9, "liquid", false, true}, {15, "solid", true, true},
	} {
		got := stateFromFlags(tc.flags)
		if got.Platform != runtime.GOOS || got.Mode != tc.mode || got.ReduceTransparency != tc.transparency || got.ReduceMotion != tc.motion {
			t.Fatalf("flags %d: %+v", tc.flags, got)
		}
	}
}
