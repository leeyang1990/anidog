package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/anidog/anidog-go/internal/model"
	streamrulesvc "github.com/anidog/anidog-go/internal/service/streamrule"
)

type StreamRuleHandler struct {
	svc *streamrulesvc.Service
}

func NewStreamRuleHandler(svc *streamrulesvc.Service) *StreamRuleHandler {
	return &StreamRuleHandler{svc: svc}
}

func (h *StreamRuleHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rules := rg.Group("/stream-rules")
	rules.GET("", h.List)
	rules.GET("/export", h.Export)
	rules.POST("/import", h.Import)
	rules.GET("/:id", h.Get)
	rules.POST("/", h.Create)
	rules.PUT("/:id", h.Update)
	rules.DELETE("/:id", h.Delete)
	rules.POST("/:id/test", h.Test)
}

func (h *StreamRuleHandler) List(c *gin.Context) {
	var enabled *bool
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		val := enabledStr == "true"
		enabled = &val
	}

	rules, err := h.svc.List(c.Request.Context(), enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "获取流媒体规则列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *StreamRuleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	rule, err := h.svc.Get(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "流媒体规则不存在"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *StreamRuleHandler) Create(c *gin.Context) {
	var req model.StreamRuleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	rule := model.StreamRule{
		Name:              req.Name,
		DisplayName:       req.DisplayName,
		BaseURL:           req.BaseURL,
		SearchURL:         req.SearchURL,
		SearchListXPath:   req.SearchListXPath,
		SearchNameXPath:   req.SearchNameXPath,
		SearchResultXPath: req.SearchResultXPath,
		ChapterResultXPath: req.ChapterResultXPath,
		Enabled:           true,
	}

	if req.Version != nil {
		rule.Version = *req.Version
	}
	if req.APILevel != nil {
		rule.APILevel = *req.APILevel
	}
	if req.UsePost != nil {
		rule.UsePost = *req.UsePost
	}
	if req.UserAgent != nil {
		rule.UserAgent = req.UserAgent
	}
	if req.Referer != nil {
		rule.Referer = req.Referer
	}
	if req.UseWebview != nil {
		rule.UseWebview = *req.UseWebview
	}
	if req.MultiSources != nil {
		rule.MultiSources = *req.MultiSources
	}
	if req.ChapterRoadsXPath != nil {
		rule.ChapterRoadsXPath = req.ChapterRoadsXPath
	}
	if req.AntiCrawlerConfig != nil {
		rule.AntiCrawlerConfig = req.AntiCrawlerConfig
	}
	if req.Headers != nil {
		rule.Headers = req.Headers
	}
	if req.Cookies != nil {
		rule.Cookies = req.Cookies
	}

	if err := h.svc.Create(c.Request.Context(), &rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建流媒体规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *StreamRuleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	var req model.StreamRuleUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Version != nil {
		updates["version"] = *req.Version
	}
	if req.APILevel != nil {
		updates["api_level"] = *req.APILevel
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.SearchURL != nil {
		updates["search_url"] = *req.SearchURL
	}
	if req.UsePost != nil {
		updates["use_post"] = *req.UsePost
	}
	if req.UserAgent != nil {
		updates["user_agent"] = *req.UserAgent
	}
	if req.Referer != nil {
		updates["referer"] = *req.Referer
	}
	if req.UseWebview != nil {
		updates["use_webview"] = *req.UseWebview
	}
	if req.MultiSources != nil {
		updates["multi_sources"] = *req.MultiSources
	}
	if req.SearchListXPath != nil {
		updates["search_list_xpath"] = *req.SearchListXPath
	}
	if req.SearchNameXPath != nil {
		updates["search_name_xpath"] = *req.SearchNameXPath
	}
	if req.SearchResultXPath != nil {
		updates["search_result_xpath"] = *req.SearchResultXPath
	}
	if req.ChapterRoadsXPath != nil {
		updates["chapter_roads_xpath"] = *req.ChapterRoadsXPath
	}
	if req.ChapterResultXPath != nil {
		updates["chapter_result_xpath"] = *req.ChapterResultXPath
	}
	if req.AntiCrawlerConfig != nil {
		updates["anti_crawler_config"] = *req.AntiCrawlerConfig
	}
	if req.Headers != nil {
		updates["headers"] = *req.Headers
	}
	if req.Cookies != nil {
		updates["cookies"] = *req.Cookies
	}

	rule, err := h.svc.Update(c.Request.Context(), uint(id), updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "更新流媒体规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *StreamRuleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除流媒体规则失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "删除成功"})
}

func (h *StreamRuleHandler) Import(c *gin.Context) {
	var rawRules []map[string]interface{}
	if err := c.ShouldBindJSON(&rawRules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: 期望 JSON 数组, " + err.Error()})
		return
	}

	result := h.svc.ImportKazumiRules(c.Request.Context(), rawRules)
	c.JSON(http.StatusOK, result)
}

func (h *StreamRuleHandler) Export(c *gin.Context) {
	rules, err := h.svc.Export(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "导出流媒体规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *StreamRuleHandler) Test(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	var req struct {
		Keyword string `json:"keyword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: keyword 必填"})
		return
	}

	results, err := h.svc.TestRule(c.Request.Context(), uint(id), req.Keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "规则测试失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rule_id": id, "keyword": req.Keyword, "results": results})
}
