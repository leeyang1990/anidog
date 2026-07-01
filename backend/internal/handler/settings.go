package handler

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	settingsvc "github.com/anidog/anidog-go/internal/service/setting"
)

type SettingsHandler struct {
	svc  *settingsvc.Service
	deps SystemInfoDeps
}

func NewSettingsHandler(svc *settingsvc.Service) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// WithSystemDeps 注入系统信息探针（DB 连接池 + qBit 在线探测）。
// 可选 —— 不调用时系统信息里的 database/qbittorrent 字段返回"未连接/离线"。
func (h *SettingsHandler) WithSystemDeps(deps SystemInfoDeps) *SettingsHandler {
	h.deps = deps
	return h
}

func (h *SettingsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	settings := rg.Group("/settings")
	{
		settings.GET("", h.GetSettings)
		settings.PUT("", h.UpdateSettings)
		settings.GET("/system", h.GetSystemInfo)
		settings.POST("/test-proxy", h.TestProxy)
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
		"http_proxy":            true,
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
	c.JSON(http.StatusOK, getSystemInfo(h.svc.Config().ProjectVersion, h.deps))
}

// TestProxy 用传入的代理 URL 尝试访问 Bangumi API，验证代理是否可用。
// Body: {"proxy": "http://host.docker.internal:7890"}  (proxy 为空则走直连)
// 返回：{"ok": true|false, "latency_ms": int, "error": string}
func (h *SettingsHandler) TestProxy(c *gin.Context) {
	var req struct {
		Proxy string `json:"proxy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	transport := &http.Transport{}
	if strings.TrimSpace(req.Proxy) != "" {
		proxyURL, err := url.Parse(req.Proxy)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "代理 URL 格式错误：" + err.Error()})
			return
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	// 用 Bangumi API 根路径做健康探测，简单快速且与主要用途一致
	target := "https://api.bgm.tv/"
	reqHTTP, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	reqHTTP.Header.Set("User-Agent", "AniDog/1.0 (proxy-test)")

	start := time.Now()
	resp, err := client.Do(reqHTTP)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "latency_ms": latency, "error": err.Error(), "target": target})
		return
	}
	resp.Body.Close()

	c.JSON(http.StatusOK, gin.H{"ok": true, "latency_ms": latency, "status": resp.StatusCode, "target": target})
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
