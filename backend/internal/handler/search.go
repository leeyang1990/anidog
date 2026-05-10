package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/anidog/anidog-go/internal/service"
)

type SearchHandler struct {
	bangumiSvc *service.BangumiService
}

func NewSearchHandler(bangumiSvc *service.BangumiService) *SearchHandler {
	return &SearchHandler{bangumiSvc: bangumiSvc}
}

func (h *SearchHandler) RegisterRoutes(rg *gin.RouterGroup) {
	search := rg.Group("/search")
	search.GET("", h.Search)
	search.POST("/collect-season", h.CollectSeason)
	search.POST("/collect", h.CollectSeason)
}

func (h *SearchHandler) Search(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "keyword 参数不能为空"})
		return
	}

	// 使用带回退机制的搜索
	results, err := h.bangumiSvc.SearchAnimeWithFallback(c.Request.Context(), keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"keyword": keyword,
		"results": results,
	})
}

func (h *SearchHandler) CollectSeason(c *gin.Context) {
	var req struct {
		AnimeID uint   `json:"anime_id" binding:"required"`
		Link    string `json:"link" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"anime_id": req.AnimeID,
		"link":     req.Link,
		"message":  "整季收集功能开发中",
	})
}
