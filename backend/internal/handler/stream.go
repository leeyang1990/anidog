package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/stream"
	streamrulesvc "github.com/anidog/anidog-go/internal/service/streamrule"
)

type StreamHandler struct {
	ruleSvc   *streamrulesvc.Service
	streamMgr *stream.StreamManager
	dlSvc     *dlservice.Service
}

func NewStreamHandler(ruleSvc *streamrulesvc.Service, streamMgr *stream.StreamManager, dlSvc *dlservice.Service) *StreamHandler {
	return &StreamHandler{ruleSvc: ruleSvc, streamMgr: streamMgr, dlSvc: dlSvc}
}

func (h *StreamHandler) RegisterRoutes(rg *gin.RouterGroup) {
	s := rg.Group("/stream")
	s.GET("/search", h.Search)
	s.GET("/auto-match", h.AutoMatch)
	s.GET("/episodes", h.Episodes)
	s.POST("/download", h.Download)
	s.POST("/download/batch", h.BatchDownload)
	s.PUT("/download/:id/cancel", h.CancelDownload)
}

func (h *StreamHandler) Search(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "keyword 参数不能为空"})
		return
	}

	ruleIDStr := c.Query("rule_id")
	if ruleIDStr != "" {
		ruleID, _ := strconv.ParseUint(ruleIDStr, 10, 64)
		rule, err := h.ruleSvc.Get(c.Request.Context(), uint(ruleID))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"detail": "规则不存在"})
			return
		}
		results, err := h.streamMgr.SearchAnime(c.Request.Context(), rule, keyword)
		if err != nil {
			zap.L().Error("流媒体搜索失败", zap.String("rule", rule.Name), zap.Error(err))
			c.JSON(http.StatusOK, gin.H{"keyword": keyword, "rule_id": ruleIDStr, "results": []interface{}{}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"keyword": keyword, "rule_id": ruleIDStr, "results": results})
		return
	}

	rules, _ := h.ruleSvc.List(c.Request.Context(), boolPtr(true))
	var allResults []stream.SearchResult
	for _, rule := range rules {
		results, err := h.streamMgr.SearchAnime(c.Request.Context(), &rule, keyword)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}

	c.JSON(http.StatusOK, gin.H{"keyword": keyword, "results": allResults})
}

// AutoMatch 并发搜索所有启用的规则，按规则分组返回结果。
type matchResult struct {
	RuleID   uint                  `json:"rule_id"`
	RuleName string                `json:"rule_name"`
	Results  []stream.SearchResult `json:"results"`
}

func (h *StreamHandler) AutoMatch(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "keyword 参数不能为空"})
		return
	}

	ctx := c.Request.Context()
	rules, err := h.ruleSvc.List(ctx, boolPtr(true))
	if err != nil || len(rules) == 0 {
		c.JSON(http.StatusOK, gin.H{"matches": []interface{}{}})
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	var mu sync.Mutex
	var matches []matchResult

	for i := range rules {
		rule := &rules[i]
		g.Go(func() error {
			results, searchErr := h.streamMgr.SearchAnime(gctx, rule, keyword)
			if searchErr != nil {
				zap.L().Debug("规则搜索失败", zap.String("rule", rule.Name), zap.Error(searchErr))
				return nil
			}
			if len(results) > 0 {
				// 按匹配度排序，最佳候选排首位
				stream.SortResultsByMatch(keyword, results)
				mu.Lock()
				matches = append(matches, matchResult{
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Results:  results,
				})
				mu.Unlock()
			}
			return nil
		})
	}

	// 带超时的等待
	done := make(chan struct{})
	go func() {
		g.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
	}

	// 结果多的排前面
	sort.Slice(matches, func(i, j int) bool {
		return len(matches[i].Results) > len(matches[j].Results)
	})

	c.JSON(http.StatusOK, gin.H{"matches": matches})
}

func (h *StreamHandler) Episodes(c *gin.Context) {
	detailURL := c.Query("detail_url")
	ruleIDStr := c.Query("rule_id")

	if detailURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "detail_url 参数不能为空"})
		return
	}
	if ruleIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "rule_id 参数不能为空"})
		return
	}

	ruleID, _ := strconv.ParseUint(ruleIDStr, 10, 64)
	rule, err := h.ruleSvc.Get(c.Request.Context(), uint(ruleID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "规则不存在"})
		return
	}

	episodes, err := h.streamMgr.GetEpisodes(c.Request.Context(), rule, detailURL)
	if err != nil {
		zap.L().Error("获取集数列表失败", zap.String("url", detailURL), zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"detail_url": detailURL, "episodes": []interface{}{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail_url": detailURL, "episodes": episodes})
}

type streamDownloadRequest struct {
	RuleID        uint   `json:"rule_id" binding:"required"`
	EpisodeURL    string `json:"episode_url" binding:"required"`
	AnimeName     string `json:"anime_name" binding:"required"`
	EpisodeNumber int    `json:"episode_number" binding:"required"`
	SavePath      string `json:"save_path"`
}

func (h *StreamHandler) Download(c *gin.Context) {
	var req streamDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	rule, err := h.ruleSvc.Get(c.Request.Context(), req.RuleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "规则不存在"})
		return
	}

	epName := fmt.Sprintf("%s - 第%d集", req.AnimeName, req.EpisodeNumber)
	task := &dlservice.Task{
		Name:          epName,
		URL:           req.EpisodeURL,
		DownloadType:  model.DownloadTypeStream,
		SavePath:      req.SavePath,
		Source:        dlservice.SourceStream,
		AnimeName:     req.AnimeName,
		StreamRuleID:  &req.RuleID,
		StreamRule:    rule,
		EpisodeNumber: &req.EpisodeNumber,
	}

	dl, err := h.dlSvc.Create(c.Request.Context(), task)
	if err != nil {
		zap.L().Error("创建流媒体下载任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建下载任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "下载任务已创建", "id": dl.ID, "task_id": dl.TorrentID})
}

type batchEpisodeInfo struct {
	URL    string `json:"url" binding:"required"`
	Name   string `json:"name"`
	Number int    `json:"number" binding:"required"`
}

type batchDownloadRequest struct {
	RuleID    uint               `json:"rule_id" binding:"required"`
	Episodes  []batchEpisodeInfo `json:"episodes" binding:"required,min=1"`
	AnimeName string             `json:"anime_name" binding:"required"`
	SavePath  string             `json:"save_path"`
	AnimeID   *uint              `json:"anime_id"`
}

func (h *StreamHandler) BatchDownload(c *gin.Context) {
	var req batchDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	rule, err := h.ruleSvc.Get(c.Request.Context(), req.RuleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "规则不存在"})
		return
	}

	var taskIDs []string
	for _, ep := range req.Episodes {
		name := ep.Name
		if name == "" {
			name = fmt.Sprintf("%s - 第%d集", req.AnimeName, ep.Number)
		}

		epNum := ep.Number
			task := &dlservice.Task{
				Name:          name,
				URL:           ep.URL,
				DownloadType:  model.DownloadTypeStream,
				SavePath:      req.SavePath,
				Source:        dlservice.SourceStream,
				AnimeName:     req.AnimeName,
				AnimeID:       req.AnimeID,
				StreamRuleID:  &req.RuleID,
				StreamRule:    rule,
				EpisodeNumber: &epNum,
			}

		dl, err := h.dlSvc.Create(c.Request.Context(), task)
		if err != nil {
			zap.L().Error("创建批量下载任务失败", zap.String("episode", name), zap.Error(err))
			continue
		}
		taskIDs = append(taskIDs, dl.TorrentID)
	}

	c.JSON(http.StatusOK, gin.H{"detail": "批量下载任务已创建", "count": len(taskIDs), "task_ids": taskIDs})
}

func (h *StreamHandler) CancelDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的下载 ID"})
		return
	}

	download, err := h.dlSvc.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "下载记录不存在"})
		return
	}

	if download.DownloadType != model.DownloadTypeStream {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "该下载任务不是流媒体类型"})
		return
	}

	if err := h.dlSvc.Cancel(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "取消下载失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"detail": "取消下载成功"})
}

func boolPtr(b bool) *bool { return &b }
