package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/anidog/anidog-go/internal/testutil"
)

func setupAuthSvc() *Service {
	db := testutil.InitTestDB()
	return New(db, "test-secret", 60*1e9) // 60 min
}

func TestAuthenticate_Success(t *testing.T) {
	svc := setupAuthSvc()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	svc.db.Exec("INSERT INTO user (username, password_hash, is_active) VALUES (?, ?, ?)", "testuser", string(hash), true)

	user, err := svc.Authenticate(context.Background(), "testuser", "pass123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("username = %q; want testuser", user.Username)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	svc := setupAuthSvc()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	svc.db.Exec("INSERT INTO user (username, password_hash, is_active) VALUES (?, ?, ?)", "testuser", string(hash), true)

	_, err := svc.Authenticate(context.Background(), "testuser", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticate_UserNotFound(t *testing.T) {
	svc := setupAuthSvc()
	_, err := svc.Authenticate(context.Background(), "nobody", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticate_UserDisabled(t *testing.T) {
	svc := setupAuthSvc()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	svc.db.Exec("INSERT INTO user (username, password_hash, is_active) VALUES (?, ?, ?)", "disabled", string(hash), false)

	_, err := svc.Authenticate(context.Background(), "disabled", "pass123")
	if !errors.Is(err, ErrUserDisabled) {
		t.Errorf("expected ErrUserDisabled, got %v", err)
	}
}

func TestCreateTokenPair(t *testing.T) {
	svc := setupAuthSvc()
	access, refresh, err := svc.CreateTokenPair("testuser")
	if err != nil {
		t.Fatalf("CreateTokenPair failed: %v", err)
	}
	if access == "" {
		t.Error("access token should not be empty")
	}
	if refresh == "" {
		t.Error("refresh token should not be empty")
	}
	if access == refresh {
		t.Error("access and refresh should differ")
	}
}

func TestHashPassword(t *testing.T) {
	svc := setupAuthSvc()
	hash, err := svc.HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("mypassword")); err != nil {
		t.Error("password hash mismatch")
	}
}

func TestValidateUserActive(t *testing.T) {
	svc := setupAuthSvc()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	svc.db.Exec("INSERT INTO user (username, password_hash, is_active) VALUES (?, ?, ?)", "active", string(hash), true)
	svc.db.Exec("INSERT INTO user (username, password_hash, is_active) VALUES (?, ?, ?)", "inactive", string(hash), false)

	_, err := svc.ValidateUserActive(context.Background(), "active")
	if err != nil {
		t.Errorf("active user should pass: %v", err)
	}

	_, err = svc.ValidateUserActive(context.Background(), "inactive")
	if !errors.Is(err, ErrUserDisabled) {
		t.Errorf("inactive user: expected ErrUserDisabled, got %v", err)
	}

	_, err = svc.ValidateUserActive(context.Background(), "nobody")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("missing user: expected ErrUserNotFound, got %v", err)
	}
}

func TestHasAnyUsers(t *testing.T) {
	svc := setupAuthSvc()
	if svc.HasAnyUsers(context.Background()) {
		t.Error("empty DB should have no users")
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	svc.db.Exec("INSERT INTO user (username, password_hash, is_active) VALUES (?, ?, ?)", "u1", string(hash), true)
	if !svc.HasAnyUsers(context.Background()) {
		t.Error("DB with a user should return true")
	}
}

func TestIsUsernameTaken(t *testing.T) {
	svc := setupAuthSvc()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	svc.db.Exec("INSERT INTO user (username, password_hash, is_active) VALUES (?, ?, ?)", "taken", string(hash), true)

	if !svc.IsUsernameTaken(context.Background(), "taken", 0) {
		t.Error("taken username should return true")
	}
	if svc.IsUsernameTaken(context.Background(), "free", 0) {
		t.Error("free username should return false")
	}
}

func TestCreateUser(t *testing.T) {
	svc := setupAuthSvc()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	user, err := svc.CreateUser(context.Background(), "newuser", "new@test.com", string(hash), false, true)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.Username != "newuser" {
		t.Errorf("username = %q; want newuser", user.Username)
	}
	if user.IsAdmin {
		t.Error("new user should not be admin")
	}
}
