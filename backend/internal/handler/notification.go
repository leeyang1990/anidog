package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/anidog/anidog-go/internal/model"
	notifsvc "github.com/anidog/anidog-go/internal/service/notification"
)

type NotificationHandler struct {
	notifSvc *notifsvc.Service
}

func NewNotificationHandler(notifSvc *notifsvc.Service) *NotificationHandler {
	return &NotificationHandler{notifSvc: notifSvc}
}

func (h *NotificationHandler) RegisterRoutes(rg *gin.RouterGroup) {
	notifications := rg.Group("/notifications")
	notifications.GET("", h.List)
	notifications.GET("/:id", h.Get)
	notifications.POST("", h.Create)
	notifications.PUT("/:id", h.Update)
	notifications.DELETE("/:id", h.Delete)
	notifications.POST("/:id/test", h.Test)
}

func (h *NotificationHandler) List(c *gin.Context) {
	channels, err := h.notifSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "获取通知渠道列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, channels)
}

func (h *NotificationHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	channel, err := h.notifSvc.Get(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "通知渠道不存在"})
		return
	}
	c.JSON(http.StatusOK, channel)
}

func (h *NotificationHandler) Create(c *gin.Context) {
	var req model.NotificationChannelCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	channel := model.NotificationChannel{
		Type:   req.Type,
		Name:   req.Name,
		Config: req.Config,
	}
	if req.Enabled != nil {
		channel.Enabled = *req.Enabled
	}

	if err := h.notifSvc.Create(c.Request.Context(), &channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建通知渠道失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, channel)
}

func (h *NotificationHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	var req model.NotificationChannelUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Config != nil {
		updates["config"] = *req.Config
	}

	channel, err := h.notifSvc.Update(c.Request.Context(), uint(id), updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "更新通知渠道失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, channel)
}

func (h *NotificationHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	if err := h.notifSvc.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除通知渠道失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "删除成功"})
}

func (h *NotificationHandler) Test(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	if err := h.notifSvc.Test(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "通知测试失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "通知测试成功"})
}
