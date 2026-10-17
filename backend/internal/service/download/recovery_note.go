package download

import (
	"context"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

// Keep the original attempt error and report the latest recovery separately.
func (s *Service) annotateRecovery(ctx context.Context, rows []model.Download) {
	ids := []uint{}
	for _, row := range rows {
		if row.Status == model.DownloadStatusFailed && row.AnimeID != nil && row.EpisodeNumber != nil {
			ids = append(ids, *row.AnimeID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var diagnoses []model.OrchestratorDiagnosis
	if s.db.WithContext(ctx).Where("anime_id IN ? AND checked_at >= ?", ids, time.Now().Add(-2*time.Hour)).Order("checked_at DESC").Find(&diagnoses).Error != nil {
		return
	}
	for i := range rows {
		row := &rows[i]
		if row.Status != model.DownloadStatusFailed || row.AnimeID == nil || row.EpisodeNumber == nil {
			continue
		}
		for _, d := range diagnoses {
			if d.AnimeID != *row.AnimeID || d.EpisodeNumber != *row.EpisodeNumber || !d.CheckedAt.After(row.UpdatedAt) {
				continue
			}
			if row.DownloadType == model.DownloadTypeStream && d.SourceType != SourceStream {
				continue
			}
			row.RecoveryNote = d.Reason
			break
		}
	}
}
