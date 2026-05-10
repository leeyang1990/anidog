package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	settingsvc "github.com/anidog/anidog-go/internal/service/setting"
)

type SettingsHandler struct {
	svc *settingsvc.Service
}

func NewSettingsHandler(svc *settingsvc.Service) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

func (h *SettingsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	settings := rg.Group("/settings")
	{
		settings.GET("", h.GetSettings)
		settings.PUT("", h.UpdateSettings)
		settings.GET("/system", h.GetSystemInfo)
	}
	system := rg.Group("/system")
	{
		system.GET("/info", h.GetSystemInfo)
	}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	cfg := h.svc.Config()

	settings := gin.H{
		"project_name":             cfg.ProjectName,
		"project_version":          cfg.ProjectVersion,
		"downloader_type":          cfg.DownloaderType,
		"downloader_host":          cfg.DownloaderHost,
		"media_root":               cfg.MediaRoot,
		"rss_check_interval":       cfg.RSSCheckInterval,
		"enable_notifications":     cfg.EnableNotifications,
		"log_level":                cfg.LogLevel,
		"cors_hosts":               cfg.CORSHosts,
		"rename_method":            cfg.RenameMethod,
		"rename_interval":          cfg.RenameInterval,
		"language":                 cfg.Language,
		"enable_scheduler":         cfg.EnableScheduler,
		"ffmpeg_path":              cfg.FFMPEGPath,
		"stream_download_dir":      cfg.StreamDownloadDir,
		"stream_max_concurrent":    cfg.StreamMaxConcurrent,
		"rod_headless":             cfg.RodHeadless,
		"stream_intercept_timeout": cfg.StreamInterceptTimeout,
		"bangumi_api_url":          cfg.BangumiAPIURL,
		"http_proxy":               cfg.HTTPProxy,
	}

	// DB 中的持久化设置覆盖 config 默认值
	if overrides, err := h.svc.GetAll(c.Request.Context()); err == nil {
		for k, v := range overrides {
			settings[k] = v
		}
	}

	c.JSON(http.StatusOK, settings)
}

func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 允许写入的 key 白名单
	allowed := map[string]bool{
		"download_dir":          true,
		"media_root":            true,
		"max_concurrent":        true,
		"stream_max_concurrent": true,
		"rename_method":         true,
		"rename_interval":       true,
		"rss_check_interval":    true,
	}

	pairs := make(map[string]string)
	for k, v := range req {
		// 允许白名单或 download.* 前缀的所有 key
		if !allowed[k] && !strings.HasPrefix(k, "download.") {
			continue
		}
		pairs[k] = toString(v)
	}

	if len(pairs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有可保存的字段"})
		return
	}

	if err := h.svc.SetMulti(c.Request.Context(), pairs); err != nil {
		zap.L().Error("保存设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已保存", "updated": pairs})
}

func (h *SettingsHandler) GetSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, getSystemInfo(h.svc.Config().ProjectVersion))
}

func toString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		return ""
	}
}
