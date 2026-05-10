package user

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetByUsername looks up a user by username.
func (s *Service) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return &user, nil
}

// GetByID looks up a user by ID.
func (s *Service) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return &user, nil
}

// List lists users with offset/limit.
func (s *Service) List(ctx context.Context, offset, limit int) ([]model.User, error) {
	var users []model.User
	if err := s.db.WithContext(ctx).Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("查询用户失败")
	}
	return users, nil
}

// Update updates a user's fields.
func (s *Service) Update(ctx context.Context, id uint, updates map[string]interface{}) (*model.User, error) {
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新用户失败: %w", err)
	}
	return s.GetByID(ctx, id)
}

// Delete deletes a user by ID.
func (s *Service) Delete(ctx context.Context, id uint) error {
	if err := s.db.WithContext(ctx).Delete(&model.User{}, id).Error; err != nil {
		return fmt.Errorf("删除用户失败")
	}
	return nil
}

// ChangePassword verifies old password and sets new one.
func (s *Service) ChangePassword(ctx context.Context, userID uint, oldPassword, hashedNewPassword string) error {
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password_hash", hashedNewPassword).Error
}
