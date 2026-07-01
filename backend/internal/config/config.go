package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Version 由构建时通过 -ldflags 注入：
//
//	go build -ldflags "-X 'github.com/anidog/anidog-go/internal/config.Version=v1.2.3'"
//
// 默认 "dev"。非 "dev" 时会覆盖配置里的 project_version，让 /system/info
// 显示真实发布版本（CI 把 git tag 注进来）。
var Version = "dev"

type Config struct {
	ProjectName              string
	ProjectVersion           string
	DatabaseURL              string
	SecretKey                string
	AccessTokenExpireMinutes int
	AccessTokenExpireDuration time.Duration

	// 数据库连接池配置（仅 PostgreSQL 有效）
	DBMaxOpenConns   int
	DBMaxIdleConns   int

	// 下载器配置
	DownloaderType     string
	DownloaderHost     string
	DownloaderUsername string
	DownloaderPassword string

	// 媒体目录
	MediaRoot string

	// RSS
	RSSCheckInterval int

	// 通知
	EnableNotifications bool

	// 日志
	LogLevel string

	// CORS
	CORSHosts []string

	// 重命名
	RenameMethod   string
	RenameInterval int

	// 语言
	Language string

	// 调度器
	EnableScheduler bool

	// 代理
	HTTPProxy string

	// Bangumi
	BangumiAPIURL      string
	BangumiAccessToken string

	// 默认规则配置
	EnableDefaultRules bool   // 是否启用默认规则作为回退
	DefaultRuleName    string // 优先使用的默认规则名称

	// 流媒体
	FFMPEGPath             string
	StreamDownloadDir      string
	StreamMaxConcurrent    int
	RodHeadless            bool
	StreamInterceptTimeout int
}

func Load() *Config {
	configName := os.Getenv("CONFIG_NAME")
	if configName == "" {
		configName = ".env"
	}
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./backend-go")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 默认值
	viper.SetDefault("app_name", "AniDog")
	viper.SetDefault("project_version", "1.0.0")
	viper.SetDefault("database_url", "sqlite:///./mikanani.db")
	viper.SetDefault("secret_key", "supersecretkey")
	viper.SetDefault("access_token_expire_minutes", 1440)
	viper.SetDefault("downloader_type", "qbittorrent")
	viper.SetDefault("downloader_host", "http://localhost:8080")
	viper.SetDefault("downloader_username", "admin")
	viper.SetDefault("downloader_password", "adminadmin")
	viper.SetDefault("rss_check_interval", 30)
	viper.SetDefault("log_level", "INFO")
	viper.SetDefault("cors_hosts", []string{"http://localhost:3000", "http://localhost:5173"})
	viper.SetDefault("tmdb_language", "zh-CN")
	viper.SetDefault("tmdb_base_url", "https://api.themoviedb.org/3")
	viper.SetDefault("rename_method", "pn")
	viper.SetDefault("rename_interval", 300)
	viper.SetDefault("language", "zh")
	viper.SetDefault("enable_scheduler", true)
	viper.SetDefault("bangumi_api_url", "https://api.bgm.tv")
	viper.SetDefault("ffmpeg_path", "ffmpeg")
	viper.SetDefault("stream_max_concurrent", 3)
	viper.SetDefault("rod_headless", true)
	viper.SetDefault("stream_intercept_timeout", 30)

	viper.SetDefault("enable_default_rules", true)
	viper.SetDefault("default_rule_name", "mikan")

	viper.SetDefault("db_max_open_conns", 25)
	viper.SetDefault("db_max_idle_conns", 5)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			zap.L().Warn("读取配置文件失败", zap.Error(err))
		}
	}

	cfg := &Config{
		ProjectName:              viper.GetString("app_name"),
		ProjectVersion:           resolveVersion(),
		DatabaseURL:              viper.GetString("database_url"),
		SecretKey:                viper.GetString("secret_key"),
		AccessTokenExpireMinutes: viper.GetInt("access_token_expire_minutes"),
		DBMaxOpenConns:          viper.GetInt("db_max_open_conns"),
		DBMaxIdleConns:          viper.GetInt("db_max_idle_conns"),
		DownloaderType:           viper.GetString("downloader_type"),
		DownloaderHost:           viper.GetString("downloader_host"),
		DownloaderUsername:        viper.GetString("downloader_username"),
		DownloaderPassword:        viper.GetString("downloader_password"),
		RSSCheckInterval:         viper.GetInt("rss_check_interval"),
		LogLevel:                 viper.GetString("log_level"),
		CORSHosts:               viper.GetStringSlice("cors_hosts"),
		RenameMethod:             viper.GetString("rename_method"),
		RenameInterval:            viper.GetInt("rename_interval"),
		Language:                 viper.GetString("language"),
		EnableScheduler:          viper.GetBool("enable_scheduler"),
		BangumiAPIURL:            viper.GetString("bangumi_api_url"),
		FFMPEGPath:               viper.GetString("ffmpeg_path"),
		StreamMaxConcurrent:       viper.GetInt("stream_max_concurrent"),
		RodHeadless:              viper.GetBool("rod_headless"),
		StreamInterceptTimeout:    viper.GetInt("stream_intercept_timeout"),
		EnableNotifications:       viper.GetBool("enable_notifications"),
		EnableDefaultRules:        viper.GetBool("enable_default_rules"),
		DefaultRuleName:           viper.GetString("default_rule_name"),
		MediaRoot:                 viper.GetString("media_root"),
		StreamDownloadDir:         viper.GetString("stream_download_dir"),
	}

	cfg.AccessTokenExpireDuration = time.Duration(cfg.AccessTokenExpireMinutes) * time.Minute

	return cfg
}

// resolveVersion 决定最终展示的版本号：
// 构建时若用 ldflags 注入了 Version（非 "dev"），优先用它（= git tag）；
// 否则退回配置文件/默认值里的 project_version。
func resolveVersion() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	return viper.GetString("project_version")
}
