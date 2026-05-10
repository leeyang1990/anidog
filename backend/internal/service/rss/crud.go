package rss

import (
	"context"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

// CRUDService handles RSS feed and rule persistence.
type CRUDService struct {
	db *gorm.DB
}

func NewCRUDService(db *gorm.DB) *CRUDService {
	return &CRUDService{db: db}
}

func (s *CRUDService) ListFeeds(ctx context.Context) ([]model.RSSFeed, error) {
	var feeds []model.RSSFeed
	err := s.db.WithContext(ctx).Preload("Rules").Order("id DESC").Find(&feeds).Error
	return feeds, err
}

func (s *CRUDService) GetFeed(ctx context.Context, id uint) (*model.RSSFeed, error) {
	var feed model.RSSFeed
	if err := s.db.WithContext(ctx).Preload("Rules").First(&feed, id).Error; err != nil {
		return nil, err
	}
	return &feed, nil
}

func (s *CRUDService) CreateFeed(ctx context.Context, feed *model.RSSFeed) error {
	if err := s.db.WithContext(ctx).Create(feed).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Preload("Rules").First(feed, feed.ID).Error
}

func (s *CRUDService) UpdateFeed(ctx context.Context, id uint, updates map[string]interface{}) (*model.RSSFeed, error) {
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&model.RSSFeed{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var feed model.RSSFeed
	if err := s.db.WithContext(ctx).Preload("Rules").First(&feed, id).Error; err != nil {
		return nil, err
	}
	return &feed, nil
}

func (s *CRUDService) DeleteFeed(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rss_feed_id = ?", id).Delete(&model.RSSRule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("rss_feed_id = ?", id).Delete(&model.RSSEntry{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.RSSFeed{}, id).Error
	})
}

func (s *CRUDService) ListRules(ctx context.Context, feedID uint) ([]model.RSSRule, error) {
	var rules []model.RSSRule
	err := s.db.WithContext(ctx).Where("rss_feed_id = ?", feedID).Order("id ASC").Find(&rules).Error
	return rules, err
}

func (s *CRUDService) CreateRule(ctx context.Context, rule *model.RSSRule) error {
	return s.db.WithContext(ctx).Create(rule).Error
}

func (s *CRUDService) UpdateRule(ctx context.Context, ruleID uint, updates map[string]interface{}) (*model.RSSRule, error) {
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&model.RSSRule{}).Where("id = ?", ruleID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var rule model.RSSRule
	if err := s.db.WithContext(ctx).First(&rule, ruleID).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *CRUDService) DeleteRule(ctx context.Context, ruleID uint) error {
	return s.db.WithContext(ctx).Delete(&model.RSSRule{}, ruleID).Error
}

func (s *CRUDService) GetEntries(ctx context.Context, feedID uint) ([]model.RSSEntry, error) {
	var entries []model.RSSEntry
	err := s.db.WithContext(ctx).Where("rss_feed_id = ?", feedID).Order("published DESC").Find(&entries).Error
	return entries, err
}
