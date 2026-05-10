package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/anidog/anidog-go/internal/config"
)

type DefaultRulesHandler struct {
	cfg *config.Config
}

func NewDefaultRulesHandler(cfg *config.Config) *DefaultRulesHandler {
	return &DefaultRulesHandler{cfg: cfg}
}

func (h *DefaultRulesHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rules := rg.Group("/default-rules")
	rules.GET("", h.ListRules)
	rules.GET("/:name/test", h.TestRule)
}

// ListRules 获取所有可用的默认规则
func (h *DefaultRulesHandler) ListRules(c *gin.Context) {
	rules := make([]map[string]interface{}, 0)

	for i := range config.DefaultRules {
		rule := config.DefaultRules[i]
		ruleInfo := map[string]interface{}{
			"name":        rule.Name,
			"description": rule.Description,
			"base_url":    rule.BaseURL,
			"enabled":     rule.Name == h.cfg.DefaultRuleName,
		}
		rules = append(rules, ruleInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"rules":                rules,
		"enabled":              h.cfg.EnableDefaultRules,
		"current_rule":         h.cfg.DefaultRuleName,
		"total_rules_count":    len(config.DefaultRules),
	})
}

// TestRule 测试指定规则是否可用
func (h *DefaultRulesHandler) TestRule(c *gin.Context) {
	ruleName := c.Param("name")
	keyword := c.Query("keyword")
	if keyword == "" {
		keyword = "海贼王" // 默认测试关键词
	}

	rule, ok := config.GetRuleByName(ruleName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "规则不存在",
			"name":  ruleName,
		})
		return
	}

	ctx := c.Request.Context()
	timeout := 15 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := &http.Client{
		Timeout: timeout,
	}
	selector := config.NewXpathSelector(rule, client, ctx)
	results, err := selector.SearchAnime(keyword)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"rule":     ruleName,
			"keyword":  keyword,
			"success":  false,
			"error":    err.Error(),
			"results":  []map[string]interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rule":         ruleName,
		"keyword":      keyword,
		"success":      true,
		"results_count": len(results),
		"results":      results,
	})
}
