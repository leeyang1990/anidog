package notification

import (
	"context"
	"sync"

	"go.uber.org/zap"
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

// Broadcast 把一条通知并发推到所有 enabled 渠道。
//
// 设计要点：
//   - 单渠道失败仅记 warn 不阻塞其他渠道（通知是 best-effort，不应该让一个挂掉的
//     webhook 拖慢整条事件流）。
//   - 完全 fire-and-forget：调用方不等结果。但内部仍然会等所有 goroutine 跑完，
//     避免 ctx 提前 cancel 把 HTTP 请求打断（调用方应该传一个不会很快超时的 ctx）。
//   - 没有 enabled 渠道时直接 return，不打 log（这是常态，不是异常）。
func (s *Service) Broadcast(ctx context.Context, info *NotificationInfo) {
	var channels []model.NotificationChannel
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Find(&channels).Error; err != nil {
		zap.L().Warn("查询启用的通知渠道失败", zap.Error(err))
		return
	}
	if len(channels) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(ch model.NotificationChannel) {
			defer wg.Done()
			provider, err := CreateProvider(ch.Type, ch.Config)
			if err != nil {
				zap.L().Warn("构造通知 provider 失败",
					zap.Uint("channel_id", ch.ID),
					zap.String("type", ch.Type),
					zap.Error(err))
				return
			}
			if err := provider.Send(ctx, info); err != nil {
				zap.L().Warn("发送通知失败",
					zap.Uint("channel_id", ch.ID),
					zap.String("type", ch.Type),
					zap.String("name", ch.Name),
					zap.Error(err))
				return
			}
			zap.L().Info("通知已发送",
				zap.Uint("channel_id", ch.ID),
				zap.String("type", ch.Type),
				zap.String("name", ch.Name))
		}(ch)
	}
	wg.Wait()
}
