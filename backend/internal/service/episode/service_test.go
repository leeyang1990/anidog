package episode

import "testing"

func TestReferenceDurationParsing(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{{"00:23:50", 1430}, {"00:07:00", 420}, {"00:41:10", 2470}, {"", 0}, {"*", 0}, {"00:70:00", 0}, {"-1:00:00", 0}, {"24:00:00", 0}} {
		if got := parseDuration(tc.value); got != tc.want {
			t.Errorf("%q=%d want %d", tc.value, got, tc.want)
		}
	}
}
