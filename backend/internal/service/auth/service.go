package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserDisabled       = errors.New("账户已被禁用")
	ErrUserNotFound       = errors.New("用户不存在")
)

type Service struct {
	db        *gorm.DB
	secretKey string
	tokenTTL  time.Duration
}

func New(db *gorm.DB, secretKey string, tokenTTL time.Duration) *Service {
	return &Service{db: db, secretKey: secretKey, tokenTTL: tokenTTL}
}

// Authenticate validates username+password and returns the user.
func (s *Service) Authenticate(ctx context.Context, username, password string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, ErrUserDisabled
	}
	return &user, nil
}

// CreateTokenPair creates an access token and a refresh token.
func (s *Service) CreateTokenPair(username string) (accessToken, refreshToken string, err error) {
	accessClaims := jwt.MapClaims{
		"sub": username,
		"exp": jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
	}
	accessTok := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, _ = accessTok.SignedString([]byte(s.secretKey))

	refreshClaims := jwt.MapClaims{
		"sub":  username,
		"type": "refresh",
		"exp":  jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
	}
	refreshTok := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, _ = refreshTok.SignedString([]byte(s.secretKey))

	return accessToken, refreshToken, nil
}

// HashPassword hashes a plaintext password.
func (s *Service) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("密码加密失败")
	}
	return string(hash), nil
}

// ValidateUserActive checks that a user exists and is active.
func (s *Service) ValidateUserActive(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, ErrUserNotFound
	}
	if !user.IsActive {
		return nil, ErrUserDisabled
	}
	return &user, nil
}

// HasAnyUsers checks if at least one user exists.
func (s *Service) HasAnyUsers(ctx context.Context) bool {
	var count int64
	s.db.WithContext(ctx).Model(&model.User{}).Count(&count)
	return count > 0
}

// HasAdmin checks if at least one admin exists.
func (s *Service) HasAdmin(ctx context.Context) bool {
	var user model.User
	return s.db.WithContext(ctx).Where("is_admin = ?", true).First(&user).Error == nil
}

// IsUsernameTaken checks if a username is already used.
func (s *Service) IsUsernameTaken(ctx context.Context, username string, excludeID uint) bool {
	var user model.User
	q := s.db.WithContext(ctx).Where("username = ?", username)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	return q.First(&user).Error == nil
}

// CreateUser creates a user with the given fields.
func (s *Service) CreateUser(ctx context.Context, username, email, hashedPassword string, isAdmin, isActive bool) (*model.User, error) {
	user := model.User{
		Username:     username,
		Email:        &email,
		PasswordHash: hashedPassword,
		IsAdmin:      isAdmin,
		IsActive:     isActive,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, errors.New("创建用户失败")
	}
	return &user, nil
}
