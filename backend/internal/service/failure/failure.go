// Package failure defines execution facts shared by recovery and source health.
// Messages are diagnostics; stable codes, rather than translated text, determine policy.
package failure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"

	"github.com/anidog/anidog-go/internal/model"
)

type Code string
type Stage string

const (
	IdentityMismatch   Code = "identity_mismatch"
	PreferenceMismatch Code = "preference_mismatch"
	InvalidMedia       Code = "invalid_media"
	SourceUnavailable  Code = "source_unavailable"
	PlayerUnavailable  Code = "player_unavailable"
	InspectionFailed   Code = "inspection_failed"
	ToolUnavailable    Code = "tool_unavailable"
	StorageUnavailable Code = "storage_unavailable"
	StateUnavailable   Code = "state_unavailable"
	Canceled           Code = "canceled"
)

const (
	Identity   Stage = "identity"
	Catalog    Stage = "catalog"
	Player     Stage = "player"
	Probe      Stage = "probe"
	Subtitles  Stage = "subtitles"
	Transfer   Stage = "transfer"
	Validation Stage = "validation"
	Archive    Stage = "archive"
	Engine     Stage = "engine"
)

type Error struct {
	Code  Code
	Stage Stage
	Err   error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Wrap preserves an already classified inner failure. A higher layer adds
// context with fmt.Errorf("...: %w", err), rather than reclassifying it.
func Wrap(stage Stage, code Code, err error) error {
	if err == nil {
		return nil
	}
	var classified *Error
	if errors.As(err, &classified) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		code = Canceled
	}
	var unavailable *exec.Error
	if errors.As(err, &unavailable) {
		code = ToolUnavailable
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EROFS) {
		code = StorageUnavailable
	}
	return &Error{Code: code, Stage: stage, Err: err}
}

type Facts struct {
	Code                Code
	Stage               Stage
	Kind                string
	AvailabilityFailure bool
}

func facts(code Code, stage Stage) Facts {
	f := Facts{Code: code, Stage: stage, Kind: model.FailureKindTransient}
	switch code {
	case IdentityMismatch, PreferenceMismatch, InvalidMedia:
		f.Kind = model.FailureKindRejected
	case PlayerUnavailable:
		f.Kind, f.AvailabilityFailure = model.FailureKindRejected, true
	case SourceUnavailable:
		f.AvailabilityFailure = true
	case ToolUnavailable, StorageUnavailable, StateUnavailable:
		f.Kind = model.FailureKindPermanent
	}
	return f
}

func Describe(err error) Facts {
	if err == nil {
		return Facts{}
	}
	var classified *Error
	if errors.As(err, &classified) {
		return facts(classified.Code, classified.Stage)
	}
	if errors.Is(err, context.Canceled) {
		return facts(Canceled, "")
	}
	return legacy(err.Error())
}

// FromRecord is the sole compatibility boundary for old persisted messages.
// Unknown future codes cannot accidentally count as a source outage.
func FromRecord(row model.Download) Facts {
	if row.FailureCode != "" {
		return facts(Code(row.FailureCode), Stage(row.FailureStage))
	}
	f := legacy(row.LastError)
	if f.Code == "" {
		f.Kind = row.FailureKind
		f.AvailabilityFailure = row.FailureKind == model.FailureKindTransient || row.FailureKind == model.FailureKindExhausted
	}
	return f
}
