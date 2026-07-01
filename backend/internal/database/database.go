package database

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
)

func initPostgreSQL(cfg *config.Config, gormLogger logger.Interface) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		zap.L().Fatal("PostgreSQL 连接失败", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		zap.L().Fatal("获取 PostgreSQL 连接失败", zap.Error(err))
	}

	// PostgreSQL 连接池配置
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)

	zap.L().Info("PostgreSQL 数据库初始化成功",
		zap.String("host", cfg.DatabaseURL),
		zap.Int("max_open_conns", cfg.DBMaxOpenConns),
		zap.Int("max_idle_conns", cfg.DBMaxIdleConns),
	)

	return db
}

func initSQLite(cfg *config.Config, gormLogger logger.Interface) *gorm.DB {
	dbPath := strings.TrimPrefix(cfg.DatabaseURL, "sqlite://")
	dbPath = strings.TrimPrefix(dbPath, "sqlite:")

	if dbPath == "" || dbPath == "./" {
		dbPath = "mikanani.db"
	}

	if idx := strings.LastIndex(dbPath, "/"); idx >= 0 {
		dir := dbPath[:idx]
		os.MkdirAll(dir, 0755)
	}

	db, err := gorm.Open(sqlite.Open(dbPath+"?_busy_timeout=5000"), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		zap.L().Fatal("SQLite 连接失败", zap.Error(err))
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	zap.L().Info("SQLite 数据库初始化成功", zap.String("path", dbPath))
	return db
}

func Init(cfg *config.Config) *gorm.DB {
	gormLogger := logger.Default.LogMode(logger.Silent)
	if strings.EqualFold(cfg.LogLevel, "DEBUG") {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	var db *gorm.DB
	switch {
	case strings.HasPrefix(cfg.DatabaseURL, "postgres://") || strings.HasPrefix(cfg.DatabaseURL, "postgresql://"):
		db = initPostgreSQL(cfg, gormLogger)
	default:
		db = initSQLite(cfg, gormLogger)
	}

	// AutoMigrate
	if err := db.AutoMigrate(
		&model.User{},
		&model.Anime{},
		&model.AnimeEpisode{},
		&model.RSSFeed{},
		&model.RSSRule{},
		&model.RSSEntry{},
		&model.Download{},
		&model.NotificationChannel{},
		&model.EpisodeNotification{},
		&model.StreamRule{},
		&model.Setting{},
		&model.OrchestratorDiagnosis{},
		&model.AbandonedTorrent{},
	); err != nil {
		zap.L().Fatal("数据库迁移失败", zap.Error(err))
	}

	zap.L().Info("数据库初始化完成")
	return db
}
