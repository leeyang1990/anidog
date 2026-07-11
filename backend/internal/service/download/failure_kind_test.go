package download

import (
	"errors"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

func TestClassifyErrorBackoffAndFinalAttempt(t *testing.T) {
	err := errors.New("视频拦截超时: context deadline exceeded")
	cases := []struct {
		retryCount int
		kind       string
		delay      time.Duration
	}{
		{0, model.FailureKindTransient, 10 * time.Minute},
		{1, model.FailureKindTransient, time.Hour},
		{2, model.FailureKindTransient, 6 * time.Hour},
		{3, model.FailureKindExhausted, 0},
	}
	for _, tc := range cases {
		kind, delay := classifyError(err, tc.retryCount)
		if kind != tc.kind || delay != tc.delay {
			t.Fatalf("retry=%d: got %s/%s, want %s/%s", tc.retryCount, kind, delay, tc.kind, tc.delay)
		}
	}
}
