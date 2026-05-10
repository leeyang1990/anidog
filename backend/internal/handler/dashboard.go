package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	dashboardsvc "github.com/anidog/anidog-go/internal/service/dashboard"
)

type DashboardHandler struct {
	dashSvc *dashboardsvc.Service
}

func NewDashboardHandler(dashSvc *dashboardsvc.Service) *DashboardHandler {
	return &DashboardHandler{dashSvc: dashSvc}
}

func (h *DashboardHandler) RegisterRoutes(rg *gin.RouterGroup) {
	dashboard := rg.Group("/dashboard")
	dashboard.GET("/stats", h.GetStats)
	dashboard.GET("/download-chart", h.GetDownloadChart)
	dashboard.GET("/recent-downloads", h.GetRecentDownloads)
	dashboard.GET("/hot-anime", h.GetHotAnime)
	dashboard.GET("", h.GetDashboard)
}

func (h *DashboardHandler) GetStats(c *gin.Context) {
	stats, err := h.dashSvc.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计数据失败"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *DashboardHandler) GetDownloadChart(c *gin.Context) {
	chart, err := h.dashSvc.GetDownloadChart(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取图表数据失败"})
		return
	}
	c.JSON(http.StatusOK, chart)
}

func (h *DashboardHandler) GetRecentDownloads(c *gin.Context) {
	downloads, err := h.dashSvc.GetRecentDownloads(c.Request.Context(), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取最近下载失败"})
		return
	}
	c.JSON(http.StatusOK, downloads)
}

func (h *DashboardHandler) GetHotAnime(c *gin.Context) {
	animes, err := h.dashSvc.GetHotAnime(c.Request.Context(), 12)
	if err != nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	c.JSON(http.StatusOK, animes)
}

func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	data, err := h.dashSvc.GetDashboard(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取仪表盘数据失败"})
		return
	}
	c.JSON(http.StatusOK, data)
}
