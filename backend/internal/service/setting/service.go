package setting

import (
	"context"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
)

type Service struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewService(cfg *config.Config, db *gorm.DB) *Service {
	return &Service{cfg: cfg, db: db}
}

func (s *Service) Config() *config.Config {
	return s.cfg
}

// GetAll 返回所有 DB 中的 key-value 设置
func (s *Service) GetAll(ctx context.Context) (map[string]string, error) {
	var items []model.Setting
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(items))
	for _, it := range items {
		out[it.Key] = it.Value
	}
	return out, nil
}

// Get 获取单个 key 的 value。不存在返回 ("", false)
func (s *Service) Get(ctx context.Context, key string) (string, bool, error) {
	var item model.Setting
	err := s.db.WithContext(ctx).Where("key = ?", key).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return item.Value, true, nil
}

// Set 设置或更新一个 key 的 value
func (s *Service) Set(ctx context.Context, key, value string) error {
	return s.db.WithContext(ctx).Save(&model.Setting{Key: key, Value: value}).Error
}

// SetMulti 批量更新
func (s *Service) SetMulti(ctx context.Context, pairs map[string]string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for k, v := range pairs {
			if err := tx.Save(&model.Setting{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
