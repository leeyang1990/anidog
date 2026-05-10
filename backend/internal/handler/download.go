package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
)

type DownloadHandler struct {
	dlSvc *dlservice.Service
}

func NewDownloadHandler(dlSvc *dlservice.Service) *DownloadHandler {
	return &DownloadHandler{dlSvc: dlSvc}
}

func (h *DownloadHandler) RegisterRoutes(rg *gin.RouterGroup) {
	downloads := rg.Group("/downloads")
	{
		downloads.GET("", h.ListDownloads)
		downloads.GET("/:id", h.GetDownload)
		downloads.POST("/", h.CreateDownload)
		downloads.POST("/:id/pause", h.PauseDownload)
		downloads.PUT("/:id/pause", h.PauseDownload)
		downloads.POST("/:id/resume", h.ResumeDownload)
		downloads.PUT("/:id/resume", h.ResumeDownload)
		downloads.DELETE("/:id", h.DeleteDownload)
		downloads.PUT("/:id/refresh", h.RefreshDownload)
		downloads.POST("/:id/retry", h.RetryDownload)
		downloads.POST("/pause-all", h.PauseAll)
		downloads.POST("/resume-all", h.ResumeAll)
	}
}

func (h *DownloadHandler) ListDownloads(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 20
	}

	status := c.Query("status")
	downloadType := c.Query("download_type")
	roadName := c.Query("road_name")
	detailURL := c.Query("detail_url")
	animeIDStr := c.Query("anime_id")
	var animeID uint
	if animeIDStr != "" {
		if id, err := strconv.ParseUint(animeIDStr, 10, 64); err == nil {
			animeID = uint(id)
		}
	}

	result, err := h.dlSvc.List(c.Request.Context(), status, downloadType, animeID, roadName, detailURL, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取下载列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":          result.Items,
		"total":          result.Total,
		"page":           page,
		"page_size":      pageSize,
		"download_speed": 0,
		"upload_speed":   0,
	})
}

func (h *DownloadHandler) GetDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的下载 ID"})
		return
	}

	download, err := h.dlSvc.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "下载任务不存在"})
		return
	}
	c.JSON(http.StatusOK, download)
}

func (h *DownloadHandler) CreateDownload(c *gin.Context) {
	var req model.DownloadCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	url := req.URL
	name := req.Name
	if req.MagnetLink != nil && *req.MagnetLink != "" {
		url = *req.MagnetLink
	}
	if req.Title != nil && *req.Title != "" {
		name = *req.Title
	}
	if name == "" {
		name = url
	}

	downloadType := req.DownloadType
	if downloadType == "" {
		downloadType = model.DownloadTypeTorrent
	}

	savePath := ""
	if req.SavePath != nil {
		savePath = *req.SavePath
	}

	task := &dlservice.Task{
		Name:          name,
		URL:           url,
		DownloadType:  downloadType,
		SavePath:      savePath,
		Source:        dlservice.SourceManual,
		AnimeID:       req.AnimeID,
		EpisodeNumber: req.EpisodeNumber,
		StreamRuleID:  req.StreamRuleID,
	}
	// 允许前端显式传 source（比如手动选 BT 种子时 source=bt）
	if req.Source != nil && *req.Source != "" {
		valid := map[string]bool{
			dlservice.SourceManual: true, dlservice.SourceBT: true,
			dlservice.SourceStream: true, dlservice.SourceRSS: true,
			dlservice.SourceBangumi: true,
		}
		if valid[*req.Source] {
			task.Source = *req.Source
		}
	}

	dl, err := h.dlSvc.Create(c.Request.Context(), task)
	if err != nil {
		zap.L().Error("创建下载任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建下载任务失败"})
		return
	}

	zap.L().Info("创建下载任务", zap.String("name", name), zap.String("type", downloadType))
	c.JSON(http.StatusCreated, dl)
}

func (h *DownloadHandler) PauseDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的下载 ID"})
		return
	}

	download, err := h.dlSvc.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "下载任务不存在"})
		return
	}

	if download.DownloadType == model.DownloadTypeStream {
		c.JSON(http.StatusBadRequest, gin.H{"error": "流媒体下载不支持暂停"})
		return
	}

	if download.Status == model.DownloadStatusPaused {
		c.JSON(http.StatusOK, download)
		return
	}

	dl, err := h.dlSvc.Pause(uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dl)
}

func (h *DownloadHandler) ResumeDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的下载 ID"})
		return
	}

	download, err := h.dlSvc.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "下载任务不存在"})
		return
	}

	if download.DownloadType == model.DownloadTypeStream {
		c.JSON(http.StatusBadRequest, gin.H{"error": "流媒体下载不支持恢复"})
		return
	}

	if download.Status != model.DownloadStatusPaused {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前状态不支持恢复"})
		return
	}

	dl, err := h.dlSvc.Resume(uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dl)
}

func (h *DownloadHandler) DeleteDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的下载 ID"})
		return
	}

	if err := h.dlSvc.Remove(uint(id), true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除下载任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "下载任务已删除"})
}

func (h *DownloadHandler) RefreshDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的下载 ID"})
		return
	}

	download, err := h.dlSvc.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "下载任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "刷新状态功能尚未实现", "download": download})
}

func (h *DownloadHandler) RetryDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的下载 ID"})
		return
	}

	download, err := h.dlSvc.Retry(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, download)
}

func (h *DownloadHandler) PauseAll(c *gin.Context) {
	count, err := h.dlSvc.PauseAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "暂停所有下载失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "已暂停所有下载", "count": count})
}

func (h *DownloadHandler) ResumeAll(c *gin.Context) {
	count, err := h.dlSvc.ResumeAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复所有下载失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "已恢复所有下载", "count": count})
}
