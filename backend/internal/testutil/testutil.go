package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
)

// InitTestDB creates an in-memory SQLite database with all migrations.
// Each call creates a unique database to avoid cross-test contamination.
func InitTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}

	db.AutoMigrate(
		&model.User{},
		&model.Anime{},
		&model.AnimeEpisode{},
		&model.RSSFeed{},
		&model.RSSRule{},
		&model.RSSEntry{},
		&model.Download{},
		&model.NotificationChannel{},
		&model.StreamRule{},
	)

	// Clean all tables for a fresh start
	db.Exec("DELETE FROM animeepisode")
	db.Exec("DELETE FROM download")
	db.Exec("DELETE FROM rssentry")
	db.Exec("DELETE FROM rssrule")
	db.Exec("DELETE FROM rssfeed")
	db.Exec("DELETE FROM notificationchannel")
	db.Exec("DELETE FROM streamrule")
	db.Exec("DELETE FROM anime")
	db.Exec("DELETE FROM user")

	return db
}

// TestConfig returns a config suitable for tests.
func TestConfig() *config.Config {
	return &config.Config{
		ProjectName:              "测试追番",
		ProjectVersion:           "0.0.1-test",
		SecretKey:                "test-secret-key",
		AccessTokenExpireMinutes: 60,
		AccessTokenExpireDuration: 60 * time.Minute,
		DownloaderType:           "qbittorrent",
		DownloaderHost:           "http://localhost:8080",
		DownloaderUsername:       "admin",
		DownloaderPassword:       "adminadmin",
		RSSCheckInterval:         30,
		LogLevel:                 "WARN",
		EnableNotifications:      true,
		EnableScheduler:          false,
		BangumiAPIURL:            "https://api.bgm.tv",
		FFMPEGPath:               "ffmpeg",
		StreamMaxConcurrent:      1,
		RodHeadless:              true,
		StreamInterceptTimeout:   10,
	}
}

// SetupRouter creates a Gin engine in test mode.
func SetupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// MakeRequest is a helper to perform an HTTP request against a Gin router.
func MakeRequest(router *gin.Engine, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ParseResponse decodes an HTTP response body into target.
func ParseResponse(w *httptest.ResponseRecorder, target interface{}) error {
	return json.NewDecoder(w.Body).Decode(target)
}

// CreateTestUser inserts a test user and returns it.
func CreateTestUser(db *gorm.DB, username, password string, isAdmin bool) *model.User {
	user := model.User{
		Username:     username,
		PasswordHash: password,
		IsAdmin:      isAdmin,
		IsActive:     true,
	}
	db.Create(&user)
	return &user
}

// CreateTestAnime inserts a test anime and returns it.
func CreateTestAnime(db *gorm.DB, title string, subscribed bool) *model.Anime {
	anime := model.Anime{
		Title:        title,
		Status:       model.AnimeStatusOngoing,
		IsSubscribed: subscribed,
	}
	db.Create(&anime)
	return &anime
}

// Ctx returns a background context.
func Ctx() context.Context {
	return context.Background()
}
