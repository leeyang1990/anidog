package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/anidog/anidog-go/internal/service/setting"
)

// IndexerHandler 提供聚合搜索接口。
type IndexerHandler struct {
	settingSvc *setting.Service
	// 注册的所有可用 Indexer（按 Name 映射）
	indexers map[string]indexer.Indexer
}

func NewIndexerHandler(settingSvc *setting.Service) *IndexerHandler {
	return &IndexerHandler{
		settingSvc: settingSvc,
		indexers: map[string]indexer.Indexer{
			"mikan":      indexer.NewMikanIndexer(),
			"dmhy":       indexer.NewDmhyIndexer(),
			"bangumimoe": indexer.NewBangumiMoeIndexer(),
			"nyaa":       indexer.NewNyaaIndexer(),
		},
	}
}

func (h *IndexerHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/indexer")
	g.POST("/search", h.Search)
	g.GET("/list", h.List)
}

type indexerSearchReq struct {
	Keyword        string                      `json:"keyword" binding:"required"`
	Indexers       []string                    `json:"indexers"`         // 空 = 使用全局启用的
	Preference     *indexer.DownloadPreference `json:"preference"`       // 可选：覆盖全局偏好做评分
	TargetEpisode  int                         `json:"target_episode"`   // 可选：过滤集数
}

func (h *IndexerHandler) Search(c *gin.Context) {
	var req indexerSearchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	names := req.Indexers
	if len(names) == 0 {
		names = h.enabledIndexerNames(c.Request.Context())
	}

	selected := make([]indexer.Indexer, 0, len(names))
	for _, n := range names {
		if ix, ok := h.indexers[n]; ok {
			selected = append(selected, ix)
		}
	}
	if len(selected) == 0 {
		c.JSON(http.StatusOK, gin.H{"candidates": []any{}, "detail": "无可用 indexer"})
		return
	}

	cands := indexer.Aggregate(c.Request.Context(), selected, req.Keyword)

	prefs := indexer.DownloadPreference{}
	if req.Preference != nil {
		prefs = *req.Preference
	}
	ranked := indexer.RankByPreference(cands, prefs, req.TargetEpisode)

	zap.L().Debug("indexer 聚合搜索",
		zap.String("keyword", req.Keyword),
		zap.Strings("indexers", names),
		zap.Int("total", len(cands)),
		zap.Int("ranked", len(ranked)))

	c.JSON(http.StatusOK, gin.H{
		"candidates": ranked,
		"total":      len(cands),
	})
}

// List 列出所有可用 indexer（不含启用状态；启用状态在 setting 里）
func (h *IndexerHandler) List(c *gin.Context) {
	type info struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	enabled := h.enabledIndexerSet(c.Request.Context())
	out := make([]info, 0, len(h.indexers))
	for name := range h.indexers {
		out = append(out, info{Name: name, Enabled: enabled[name]})
	}
	c.JSON(http.StatusOK, gin.H{"indexers": out})
}

// enabledIndexerNames 读取 setting 表中启用的 indexer 名。
// 默认启用 mikan/dmhy/bangumimoe（nyaa 默认关）
func (h *IndexerHandler) enabledIndexerNames(ctx context.Context) []string {
	m := h.enabledIndexerSet(ctx)
	out := make([]string, 0, len(m))
	for name, on := range m {
		if on {
			out = append(out, name)
		}
	}
	return out
}

func (h *IndexerHandler) enabledIndexerSet(ctx context.Context) map[string]bool {
	// 默认值
	defaults := map[string]bool{
		"mikan":      true,
		"dmhy":       true,
		"bangumimoe": true,
		"nyaa":       false,
	}
	if h.settingSvc == nil {
		return defaults
	}
	// 尝试从 setting 读
	for name := range defaults {
		key := "download.indexer_enabled." + name
		val, ok, _ := h.settingSvc.Get(ctx, key)
		if !ok {
			continue
		}
		switch val {
		case "true", "1":
			defaults[name] = true
		case "false", "0":
			defaults[name] = false
		}
	}
	return defaults
}
