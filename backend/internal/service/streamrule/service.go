package streamrule

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/stream"
)

type Service struct {
	db       *gorm.DB
	searcher Searcher
}

// Searcher is the narrow interface for stream rule search testing.
type Searcher interface {
	SearchAnime(ctx context.Context, rule *model.StreamRule, keyword string) ([]stream.SearchResult, error)
}

func NewService(db *gorm.DB, searcher Searcher) *Service {
	return &Service{db: db, searcher: searcher}
}

func (s *Service) List(ctx context.Context, enabled *bool) ([]model.StreamRule, error) {
	query := s.db.WithContext(ctx).Model(&model.StreamRule{})
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var rules []model.StreamRule
	err := query.Find(&rules).Error
	return rules, err
}

func (s *Service) Get(ctx context.Context, id uint) (*model.StreamRule, error) {
	var rule model.StreamRule
	if err := s.db.WithContext(ctx).First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *Service) Create(ctx context.Context, rule *model.StreamRule) error {
	return s.db.WithContext(ctx).Create(rule).Error
}

func (s *Service) Update(ctx context.Context, id uint, updates map[string]interface{}) (*model.StreamRule, error) {
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&model.StreamRule{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var rule model.StreamRule
	if err := s.db.WithContext(ctx).First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.StreamRule{}, id).Error
}

// ImportResult holds the result of a Kazumi rule import.
type ImportResult struct {
	Imported int `json:"imported"`
	Failed   int `json:"failed"`
	Total    int `json:"total"`
}

// ImportKazumiRules imports Kazumi JSON rules into the database.
func (s *Service) ImportKazumiRules(ctx context.Context, rawRules []map[string]interface{}) *ImportResult {
	result := &ImportResult{Total: len(rawRules)}
	for _, raw := range rawRules {
		mapped := model.MapKazumiRule(raw)

		data, err := json.Marshal(mapped)
		if err != nil {
			zap.L().Warn("序列化规则失败", zap.Error(err))
			result.Failed++
			continue
		}

		var rule model.StreamRule
		if err := json.Unmarshal(data, &rule); err != nil {
			zap.L().Warn("反序列化规则失败", zap.Error(err))
			result.Failed++
			continue
		}

		rule.Enabled = true
		if rule.Version == "" {
			rule.Version = "1.0"
		}

		if err := s.db.WithContext(ctx).Create(&rule).Error; err != nil {
			zap.L().Warn("导入规则失败", zap.String("name", rule.Name), zap.Error(err))
			result.Failed++
			continue
		}
		result.Imported++
	}
	return result
}

func (s *Service) Export(ctx context.Context) ([]model.StreamRule, error) {
	var rules []model.StreamRule
	err := s.db.WithContext(ctx).Find(&rules).Error
	return rules, err
}

// TestRule searches using a stream rule to verify it works.
func (s *Service) TestRule(ctx context.Context, id uint, keyword string) ([]stream.SearchResult, error) {
	rule, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.searcher == nil {
		return nil, fmt.Errorf("搜索功能未配置")
	}
	return s.searcher.SearchAnime(ctx, rule, keyword)
}
