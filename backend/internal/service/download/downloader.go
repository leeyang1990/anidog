package download

import (
	"context"

	"github.com/anidog/anidog-go/internal/model"
)

// Downloader is the narrow interface that trigger sources depend on.
// The concrete *Service satisfies this interface.
// Sources should depend on this interface, not on the full Service.
type Downloader interface {
	Create(ctx context.Context, task *Task) (*model.Download, error)
}
