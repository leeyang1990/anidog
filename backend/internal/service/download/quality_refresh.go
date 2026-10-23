package download

import (
	"context"
	"sync"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type qualityRefresh struct {
	mu      sync.Mutex
	pending map[uint]bool
	handler func(context.Context, uint) error
}

// SetMediaCompletedHandler installs a read-only library quality audit. Audits
// run after the projection commits, outside the completion lock, and coalesce
// per anime while retaining changes that arrive during an in-progress audit.
func (s *Service) SetMediaCompletedHandler(handler func(context.Context, uint) error) {
	s.qualityRefresh.mu.Lock()
	defer s.qualityRefresh.mu.Unlock()
	s.qualityRefresh.handler = handler
}

func (s *Service) requestQualityRefresh(animeID uint) {
	q := &s.qualityRefresh
	q.mu.Lock()
	if animeID == 0 || q.handler == nil {
		q.mu.Unlock()
		return
	}
	if q.pending == nil {
		q.pending = make(map[uint]bool)
	}
	if _, exists := q.pending[animeID]; exists {
		q.pending[animeID] = true
		q.mu.Unlock()
		return
	}
	q.pending[animeID] = false
	handler := q.handler
	q.mu.Unlock()
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			err := handler(ctx, animeID)
			cancel()
			if err != nil {
				zap.L().Warn("归档后质量审计等待定时恢复", zap.Uint("anime_id", animeID), zap.Error(err))
			}
			q.mu.Lock()
			if !q.pending[animeID] {
				delete(q.pending, animeID)
				q.mu.Unlock()
				return
			}
			q.pending[animeID] = false
			q.mu.Unlock()
		}
	}()
}

// An asset change invalidates the old quality conclusion even when the final
// pathname is reused. Duplicate completion keeps an unchanged asset's audit.
func invalidateChangedAsset(updates map[string]interface{}, downloadID, path string) {
	for column, value := range map[string]interface{}{
		"quality_status": model.QualityUnknown, "quality_reason": "", "quality_fingerprint": "", "quality_checked_at": nil,
	} {
		updates[column] = gorm.Expr("CASE WHEN COALESCE(animeepisode.download_id, '') <> ? OR COALESCE(animeepisode.file_path, '') <> ? THEN ? ELSE animeepisode."+column+" END", downloadID, path, value)
	}
}
