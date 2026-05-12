package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
	rssservice "github.com/anidog/anidog-go/internal/service/rss"
)

type RSSHandler struct {
	rssCrud   *rssservice.CRUDService
	rssEngine *rssservice.Engine
}

func NewRSSHandler(rssCrud *rssservice.CRUDService, rssEngine *rssservice.Engine) *RSSHandler {
	return &RSSHandler{rssCrud: rssCrud, rssEngine: rssEngine}
}

func (h *RSSHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/rss")
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.POST("/", h.Create)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.GET("/:id/rules", h.ListRules)
	g.POST("/:id/rules", h.CreateRule)
	g.PUT("/:id/rules/:ruleId", h.UpdateRule)
	g.DELETE("/:id/rules/:ruleId", h.DeleteRule)
	g.POST("/:id/check", h.ManualCheck)
	g.GET("/:id/torrents", h.TorrentPreview)
	g.POST("/test", h.TestRSS)
	g.GET("/:id/items", h.GetItems)
	g.POST("/:id/refresh", h.RefreshFeed)
}

func (h *RSSHandler) List(c *gin.Context) {
	feeds, err := h.rssCrud.ListFeeds(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取RSS订阅源列表失败"})
		return
	}
	c.JSON(http.StatusOK, feeds)
}

func (h *RSSHandler) Get(c *gin.Context) {
	id, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	feed, err := h.rssCrud.GetFeed(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RSS订阅源不存在"})
		return
	}
	c.JSON(http.StatusOK, feed)
}

func (h *RSSHandler) Create(c *gin.Context) {
	var req model.RSSFeedCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	feed := model.RSSFeed{
		Name:        req.Name,
		URL:         req.URL,
		Description: req.Description,
		Enabled:     true,
		Parser:      "mikan",
	}
	if req.Enabled != nil {
		feed.Enabled = *req.Enabled
	}
	if req.CheckInterval != nil {
		feed.CheckInterval = *req.CheckInterval
	}
	if req.Aggregate != nil {
		feed.Aggregate = *req.Aggregate
	}
	if req.Parser != nil {
		feed.Parser = *req.Parser
	}

	if err := h.rssCrud.CreateFeed(c.Request.Context(), &feed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建RSS订阅源失败"})
		return
	}
	c.JSON(http.StatusCreated, feed)
}

func (h *RSSHandler) Update(c *gin.Context) {
	id, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	var req model.RSSFeedUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.CheckInterval != nil {
		updates["check_interval"] = *req.CheckInterval
	}
	if req.Aggregate != nil {
		updates["aggregate"] = *req.Aggregate
	}
	if req.Parser != nil {
		updates["parser"] = *req.Parser
	}

	feed, err := h.rssCrud.UpdateFeed(c.Request.Context(), id, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新RSS订阅源失败"})
		return
	}
	c.JSON(http.StatusOK, feed)
}

func (h *RSSHandler) Delete(c *gin.Context) {
	id, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.rssCrud.DeleteFeed(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除RSS订阅源失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "RSS订阅源已删除"})
}

func (h *RSSHandler) ListRules(c *gin.Context) {
	feedID, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	rules, err := h.rssCrud.ListRules(c.Request.Context(), feedID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取规则列表失败"})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *RSSHandler) CreateRule(c *gin.Context) {
	feedID, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	_, err := h.rssCrud.GetFeed(c.Request.Context(), feedID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RSS订阅源不存在"})
		return
	}

	var req model.RSSRuleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	rule := model.RSSRule{
		Name:      req.Name,
		Keyword:   req.Keyword,
		IsRegex:   req.IsRegex,
		Include:   req.Include,
		Enabled:   req.Enabled,
		RSSFeedID: feedID,
	}

	if err := h.rssCrud.CreateRule(c.Request.Context(), &rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建规则失败"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *RSSHandler) UpdateRule(c *gin.Context) {
	ruleID, ok := parseUintID(c.Param("ruleId"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的规则 ID"})
		return
	}

	var req model.RSSRuleUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Keyword != nil {
		updates["keyword"] = *req.Keyword
	}
	if req.IsRegex != nil {
		updates["is_regex"] = *req.IsRegex
	}
	if req.Include != nil {
		updates["include"] = *req.Include
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	rule, err := h.rssCrud.UpdateRule(c.Request.Context(), ruleID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新规则失败"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *RSSHandler) DeleteRule(c *gin.Context) {
	ruleID, ok := parseUintID(c.Param("ruleId"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的规则 ID"})
		return
	}

	if err := h.rssCrud.DeleteRule(c.Request.Context(), ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除规则失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "规则已删除"})
}

func (h *RSSHandler) ManualCheck(c *gin.Context) {
	h.triggerCheck(c, "RSS检查")
}

func (h *RSSHandler) RefreshFeed(c *gin.Context) {
	h.triggerCheck(c, "RSS 刷新")
}

func (h *RSSHandler) triggerCheck(c *gin.Context, action string) {
	id, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	feed, err := h.rssCrud.GetFeed(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RSS订阅源不存在"})
		return
	}

	if h.rssEngine != nil {
		go func() {
			if _, err := h.rssEngine.CheckFeed(context.Background(), feed); err != nil {
				zap.L().Error(action+"失败", zap.String("feed", feed.Name), zap.Error(err))
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{"message": action + "已触发", "feed_id": feed.ID})
}

func (h *RSSHandler) TorrentPreview(c *gin.Context) {
	id, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	feed, err := h.rssCrud.GetFeed(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RSS订阅源不存在"})
		return
	}

	if h.rssEngine != nil {
		items, err := h.rssEngine.ParseFeedPreview(c.Request.Context(), feed.URL)
		if err != nil {
			zap.L().Error("RSS 预览解析失败", zap.String("url", feed.URL), zap.Error(err))
			c.JSON(http.StatusOK, []interface{}{})
			return
		}
		c.JSON(http.StatusOK, items)
		return
	}

	c.JSON(http.StatusOK, []interface{}{})
}

func (h *RSSHandler) TestRSS(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if h.rssEngine != nil {
		items, err := h.rssEngine.ParseFeedPreview(c.Request.Context(), req.URL)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"url": req.URL, "error": err.Error(), "items": []interface{}{}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": req.URL, "items": items, "count": len(items)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": req.URL, "message": "RSS 测试功能开发中"})
}

func (h *RSSHandler) GetItems(c *gin.Context) {
	id, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	entries, err := h.rssCrud.GetEntries(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RSS订阅源不存在"})
		return
	}
	c.JSON(http.StatusOK, entries)
}

// parseUintID parses a uint from a path parameter.
func parseUintID(s string) (uint, bool) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(n), true
}
