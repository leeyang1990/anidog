package download

import (
	"github.com/anidog/anidog-go/internal/model"
	"gorm.io/gorm"
)

// AvailableMedia is the shared database predicate for a usable completion.
// Completed records retain history after files disappear; only the storage
// reconciler changes MediaMissing after checking the media root's health.
// Do not stat a NAS path here: a temporary mount failure is not proof of loss.
func AvailableMedia(db *gorm.DB) *gorm.DB {
	return db.Where("status = ? AND media_missing = ?", model.DownloadStatusCompleted, false)
}
