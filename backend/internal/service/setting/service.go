package setting

import (
	"context"
	"sync"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
)

type Service struct {
	cfg       *config.Config
	db        *gorm.DB
	mu        sync.RWMutex
	callbacks map[string][]func(string)
}

func NewService(cfg *config.Config, db *gorm.DB) *Service {
	return &Service{cfg: cfg, db: db, callbacks: make(map[string][]func(string))}
}

// OnChange registers an in-process update hook for a persisted setting.
func (s *Service) OnChange(key string, callback func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks[key] = append(s.callbacks[key], callback)
}

func (s *Service) notify(key, value string) {
	s.mu.RLock()
	callbacks := append([]func(string){}, s.callbacks[key]...)
	s.mu.RUnlock()
	for _, callback := range callbacks {
		callback(value)
	}
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
	if err := s.db.WithContext(ctx).Save(&model.Setting{Key: key, Value: value}).Error; err != nil {
		return err
	}
	s.notify(key, value)
	return nil
}

// SetMulti 批量更新
func (s *Service) SetMulti(ctx context.Context, pairs map[string]string) error {
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for k, v := range pairs {
			if err := tx.Save(&model.Setting{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for key, value := range pairs {
		s.notify(key, value)
	}
	return nil
}
