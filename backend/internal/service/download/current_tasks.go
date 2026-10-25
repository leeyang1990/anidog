package download

import (
	"context"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
	"gorm.io/gorm"
)

type CurrentResult struct {
	Items        []model.Download `json:"tasks"`
	Total        int              `json:"total"`
	AttemptTotal int64            `json:"attempt_total"`
	Counts       map[string]int   `json:"counts"`
}

// CurrentTasks derives one current outcome per episode and purpose. It never
// rewrites or hides attempt history in storage. Active work and available media
// take priority over a later failed alternative; upgrades remain separate.
func (s *Service) CurrentTasks(ctx context.Context, resultClass, search string, page, pageSize int) (*CurrentResult, error) {
	var rows []model.Download
	q := `SELECT d.id,d.name,d.status,d.failure_code,d.failure_stage,d.failure_kind,d.last_error,d.verified_languages,d.media_missing,grouped.attempt_count FROM download d JOIN (
	 SELECT id, COUNT(*) OVER (PARTITION BY anime_id,episode_number,purpose,CASE WHEN anime_id IS NULL OR episode_number IS NULL OR scope='season' THEN id ELSE 0 END) AS attempt_count,
	 ROW_NUMBER() OVER (PARTITION BY anime_id,episode_number,purpose,CASE WHEN anime_id IS NULL OR episode_number IS NULL OR scope='season' THEN id ELSE 0 END ORDER BY
	 CASE WHEN status IN ('pending','downloading') THEN 0 WHEN status='completed' AND media_missing=false THEN 1 WHEN status='paused' THEN 2 WHEN status='superseded' THEN 4 ELSE 3 END, created_at DESC,id DESC) AS position FROM download
	) grouped ON grouped.id=d.id WHERE grouped.position=1 AND d.status<>'superseded' ORDER BY d.created_at DESC,d.id DESC`
	if err := s.db.WithContext(ctx).Raw(q).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := &CurrentResult{Items: []model.Download{}, Counts: map[string]int{}}
	if err := s.db.WithContext(ctx).Model(&model.Download{}).Count(&result.AttemptTotal).Error; err != nil {
		return nil, err
	}
	var filtered []model.Download
	for _, row := range rows {
		_, row.InspectionActive = s.inspectionRequests.Load(row.ID)
		row.ResultClass = failure.ResultClass(row)
		result.Counts[row.ResultClass]++
		if resultClass != "" && resultClass != "all" && row.ResultClass != resultClass && !(resultClass == "downloading" && row.ResultClass == "queued") {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(row.Name), strings.ToLower(search)) {
			continue
		}
		filtered = append(filtered, row)
	}
	result.Total = len(filtered)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 100
	}
	if page-1 > len(filtered)/pageSize {
		return result, nil
	}
	start := (page - 1) * pageSize
	if start < len(filtered) {
		end := min(start+pageSize, len(filtered))
		selected := filtered[start:end]
		ids := make([]uint, 0, len(selected))
		for _, row := range selected {
			ids = append(ids, row.ID)
		}
		var details []model.Download
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&details).Error; err != nil {
			return nil, err
		}
		byID := make(map[uint]model.Download, len(details))
		for _, row := range details {
			byID[row.ID] = row
		}
		for _, projection := range selected {
			row, ok := byID[projection.ID]
			if !ok {
				continue
			}
			row.AttemptCount = projection.AttemptCount
			if row.Scope == model.DownloadScopeSeason {
				if err := s.attemptHistoryQuery(ctx, &row).Count(&row.AttemptCount).Error; err != nil {
					return nil, err
				}
			}
			row.ResultClass = failure.ResultClass(row)
			_, row.InspectionActive = s.inspectionRequests.Load(row.ID)
			result.Items = append(result.Items, row)
		}
		s.annotateRecovery(ctx, result.Items)
	}
	return result, nil
}

func (s *Service) AttemptHistory(ctx context.Context, id uint, page, size int) (*ListResult, error) {
	var target model.Download
	if err := s.db.WithContext(ctx).First(&target, id).Error; err != nil {
		return nil, err
	}
	q := s.attemptHistoryQuery(ctx, &target)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 1000 {
		size = 100
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	if int64(page-1) > total/int64(size) {
		return &ListResult{Items: []model.Download{}, Total: total}, nil
	}
	var rows []model.Download
	if err := q.Order("created_at DESC,id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].ResultClass = failure.ResultClass(rows[i])
	}
	return &ListResult{Items: rows, Total: total}, nil
}

func (s *Service) attemptHistoryQuery(ctx context.Context, target *model.Download) *gorm.DB {
	q := s.db.WithContext(ctx).Model(&model.Download{})
	if target.Scope == model.DownloadScopeSeason && target.AnimeID != nil && target.EpisodeStart != nil && target.EpisodeEnd != nil {
		return q.Where("id=? OR (anime_id=? AND episode_number BETWEEN ? AND ? AND scope<>?)", target.ID, *target.AnimeID, *target.EpisodeStart, *target.EpisodeEnd, model.DownloadScopeSeason)
	}
	if target.AnimeID == nil || target.EpisodeNumber == nil || target.Scope == model.DownloadScopeSeason {
		return q.Where("id=?", target.ID)
	}
	return q.Where("anime_id=? AND episode_number=? AND purpose=? AND scope<>?", *target.AnimeID, *target.EpisodeNumber, target.Purpose, model.DownloadScopeSeason)
}
