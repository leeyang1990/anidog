package notification

import (
	"context"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

// Service handles notification channel persistence.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	err := s.db.WithContext(ctx).Find(&channels).Error
	return channels, err
}

func (s *Service) Get(ctx context.Context, id uint) (*model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := s.db.WithContext(ctx).First(&channel, id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (s *Service) Create(ctx context.Context, channel *model.NotificationChannel) error {
	return s.db.WithContext(ctx).Create(channel).Error
}

func (s *Service) Update(ctx context.Context, id uint, updates map[string]interface{}) (*model.NotificationChannel, error) {
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&model.NotificationChannel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var channel model.NotificationChannel
	if err := s.db.WithContext(ctx).First(&channel, id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.NotificationChannel{}, id).Error
}

// Test sends a test notification through the specified channel.
func (s *Service) Test(ctx context.Context, id uint) error {
	var channel model.NotificationChannel
	if err := s.db.WithContext(ctx).First(&channel, id).Error; err != nil {
		return err
	}

	provider, err := CreateProvider(channel.Type, channel.Config)
	if err != nil {
		return err
	}

	return provider.Test(ctx)
}
