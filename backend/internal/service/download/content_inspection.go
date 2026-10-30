package download

import (
	"context"

	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"golang.org/x/sync/singleflight"
)

// contentInspection carries measurement facts and their original producer.
// measured distinguishes fresh observations from a cached/reassessed verdict.
type contentInspection struct {
	evidence mediacheck.SubtitleEvidence
	origin   *uint
	measured bool
}

type contentInspectionGroup struct {
	flights singleflight.Group
}

// A content result can satisfy several attempts; an execution failure belongs
// only to the request that performed it. Keep request identity separate from
// the observation's origin (which may refer to an older cached measurement).
type contentInspectionRequest struct {
	refreshUnknown bool
}

type contentInspectionOutcome struct {
	request    *contentInspectionRequest
	inspection contentInspection
	err        error
}

// inspect coalesces useful content observations while preserving
// each caller's cancellation, execution result and freshness requirement. A
// remaining caller can become the next owner after an interrupted/failed run;
// its own context and retained file are then used, with no detached OCR worker.
func (g *contentInspectionGroup) inspect(ctx context.Context, key string, refreshUnknown bool, work func() (contentInspection, error)) (contentInspection, error) {
	request := &contentInspectionRequest{refreshUnknown: refreshUnknown}
	for {
		if err := ctx.Err(); err != nil {
			return contentInspection{}, err
		}
		done := g.flights.DoChan(key, func() (interface{}, error) {
			evidence, err := work()
			return contentInspectionOutcome{request: request, inspection: evidence, err: err}, nil
		})
		select {
		case <-ctx.Done():
			return contentInspection{}, ctx.Err()
		case result := <-done:
			if err := ctx.Err(); err != nil {
				return contentInspection{}, err
			}
			if result.Err != nil {
				return contentInspection{}, result.Err
			}
			outcome := result.Val.(contentInspectionOutcome)
			failed := outcome.err != nil || outcome.inspection.evidence.Retryable
			needsMeasurement := request.refreshUnknown && outcome.inspection.evidence.Language == "" && !outcome.inspection.measured
			if outcome.request != request && (failed || needsMeasurement) {
				continue
			}
			return outcome.inspection, outcome.err
		}
	}
}
