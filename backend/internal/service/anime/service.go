package anime

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, status string, subscribed bool, page, perPage int) ([]model.Anime, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.Anime{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if subscribed {
		query = query.Where("is_subscribed = ?", true)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var animes []model.Anime
	offset := (page - 1) * perPage
	if err := query.Order("id DESC").Offset(offset).Limit(perPage).Find(&animes).Error; err != nil {
		return nil, 0, err
	}
	return animes, total, nil
}

func (s *Service) Get(ctx context.Context, id uint) (*model.Anime, error) {
	var anime model.Anime
	if err := s.db.WithContext(ctx).Preload("Episodes").First(&anime, id).Error; err != nil {
		return nil, fmt.Errorf("番剧不存在")
	}
	return &anime, nil
}

func (s *Service) Create(ctx context.Context, anime *model.Anime) error {
	if anime.Status == "" {
		anime.Status = model.AnimeStatusUnknown
	}
	return s.db.WithContext(ctx).Create(anime).Error
}

func (s *Service) Update(ctx context.Context, id uint, updates map[string]interface{}) (*model.Anime, error) {
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&model.Anime{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("anime_id = ?", id).Delete(&model.AnimeEpisode{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Anime{}, id).Error
	})
}

func (s *Service) Subscribe(ctx context.Context, id uint) (*model.Anime, error) {
	anime, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !anime.IsSubscribed {
		s.db.WithContext(ctx).Model(anime).Update("is_subscribed", true)
		anime.IsSubscribed = true
	}
	return anime, nil
}

func (s *Service) Unsubscribe(ctx context.Context, id uint) (*model.Anime, error) {
	anime, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if anime.IsSubscribed {
		s.db.WithContext(ctx).Model(anime).Update("is_subscribed", false)
		anime.IsSubscribed = false
	}
	return anime, nil
}

// IsNotFound returns true if the error indicates no record was found.
func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func (s *Service) FindByBangumiID(ctx context.Context, bangumiID int) (*model.Anime, error) {
	var anime model.Anime
	if err := s.db.WithContext(ctx).Where("bangumi_id = ?", bangumiID).First(&anime).Error; err != nil {
		return nil, err
	}
	return &anime, nil
}

func (s *Service) CreateFromBangumi(ctx context.Context, bangumiID int, detail *model.BangumiAnime) (*model.Anime, error) {
	anime := model.Anime{
		BangumiID:    &bangumiID,
		IsSubscribed: true,
	}
	if detail != nil {
		anime.Title = detail.NameCN
		if anime.Title == "" {
			anime.Title = detail.Name
		}
		anime.OriginalTitle = &detail.Name
		anime.Description = &detail.Summary
		anime.CoverURL = &detail.ImageURL
		anime.BangumiRating = &detail.Rating
		if detail.EpsCount > 0 {
			anime.EpisodeCount = &detail.EpsCount
		}
		if detail.AirWeekday >= 0 {
			anime.AirWeekday = &detail.AirWeekday
		}
		// 从 air_date 提取 year
		if y := yearFromAirDate(detail.AirDate); y > 0 {
			anime.Year = &y
		}
		// 判断状态：air_date + eps*7天 < now → completed
		anime.Status = computeStatusFromAirDate(detail.AirDate, detail.EpsCount)
	} else {
		anime.Title = fmt.Sprintf("Bangumi:%d", bangumiID)
	}
	if anime.Status == "" {
		anime.Status = model.AnimeStatusUnknown
	}
	if err := s.db.WithContext(ctx).Create(&anime).Error; err != nil {
		return nil, err
	}
	return &anime, nil
}

func (s *Service) RefreshFromBangumi(ctx context.Context, id uint, detail *model.BangumiAnime) error {
	if detail == nil {
		return nil
	}
	updates := map[string]interface{}{}
	if detail.Rating > 0 {
		updates["bangumi_rating"] = detail.Rating
	}
	if detail.EpsCount > 0 {
		updates["episode_count"] = detail.EpsCount
	}
	if detail.ImageURL != "" {
		updates["cover_url"] = detail.ImageURL
	}
	if y := yearFromAirDate(detail.AirDate); y > 0 {
		updates["year"] = y
	}
	if st := computeStatusFromAirDate(detail.AirDate, detail.EpsCount); st != "" {
		updates["status"] = st
	}
	if len(updates) > 0 {
		return s.db.WithContext(ctx).Model(&model.Anime{}).Where("id = ?", id).Updates(updates).Error
	}
	return nil
}

// yearFromAirDate 从 "YYYY-MM-DD" 提取年份
func yearFromAirDate(airDate string) int {
	if len(airDate) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(airDate[:4])
	return y
}

// computeStatusFromAirDate 基于首播时间和集数估算番剧状态。
func computeStatusFromAirDate(airDate string, epsCount int) string {
	if airDate == "" {
		return model.AnimeStatusUnknown
	}
	t, err := time.Parse("2006-01-02", airDate)
	if err != nil {
		return model.AnimeStatusUnknown
	}
	now := time.Now()
	if t.After(now) {
		return model.AnimeStatusUpcoming
	}
	eps := epsCount
	if eps <= 0 {
		eps = 13 // 保守估计一季长度
	}
	// 预计完结时间 = 首播 + (集数 + 1周缓冲)*7天
	endEst := t.AddDate(0, 0, (eps+1)*7)
	if endEst.Before(now) {
		return model.AnimeStatusFinished
	}
	return model.AnimeStatusOngoing
}

func (s *Service) GetSubscribedWithBangumiID(ctx context.Context) ([]model.Anime, error) {
	var animes []model.Anime
	err := s.db.WithContext(ctx).Where("is_subscribed = ? AND bangumi_id IS NOT NULL", true).Find(&animes).Error
	return animes, err
}

// GetSubscriptionMap returns a map from BangumiID to SubStatus.
func (s *Service) GetSubscriptionMap(ctx context.Context) map[int]model.SubStatus {
	subscribed, err := s.GetSubscribedWithBangumiID(ctx)
	if err != nil {
		return nil
	}
	m := make(map[int]model.SubStatus, len(subscribed))
	for _, a := range subscribed {
		if a.BangumiID != nil {
			m[*a.BangumiID] = model.SubStatus{
				IsSubscribed: a.IsSubscribed,
				LocalID:      a.ID,
			}
		}
	}
	return m
}

func (s *Service) GetSubscribedWithAirWeekday(ctx context.Context) ([]model.Anime, error) {
	var animes []model.Anime
	err := s.db.WithContext(ctx).Where("is_subscribed = ? AND air_weekday IS NOT NULL", true).Find(&animes).Error
	return animes, err
}

func (s *Service) GetDownloads(ctx context.Context, animeID uint) ([]model.Download, error) {
	var downloads []model.Download
	err := s.db.WithContext(ctx).Where("anime_id = ?", animeID).Order("created_at DESC").Find(&downloads).Error
	return downloads, err
}

func (s *Service) UpdateOffset(ctx context.Context, id uint, episodeOffset, seasonOffset *int) (*model.Anime, error) {
	updates := map[string]interface{}{}
	if episodeOffset != nil {
		updates["episode_offset"] = *episodeOffset
	}
	if seasonOffset != nil {
		updates["season_offset"] = *seasonOffset
	}
	if len(updates) > 0 {
		s.db.WithContext(ctx).Model(&model.Anime{}).Where("id = ?", id).Updates(updates)
	}
	return s.Get(ctx, id)
}

func (s *Service) ListEpisodes(ctx context.Context, animeID uint) ([]model.AnimeEpisode, error) {
	var episodes []model.AnimeEpisode
	err := s.db.WithContext(ctx).Where("anime_id = ?", animeID).Order("episode_number ASC").Find(&episodes).Error
	return episodes, err
}

func (s *Service) CreateEpisode(ctx context.Context, animeID uint, ep *model.AnimeEpisode) error {
	ep.AnimeID = animeID
	return s.db.WithContext(ctx).Create(ep).Error
}

func (s *Service) DeleteEpisode(ctx context.Context, animeID, episodeID uint) error {
	return s.db.WithContext(ctx).Where("id = ? AND anime_id = ?", episodeID, animeID).Delete(&model.AnimeEpisode{}).Error
}
