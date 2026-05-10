package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/anidog/anidog-go/internal/model"
	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	bangumisvc "github.com/anidog/anidog-go/internal/service/bangumi"
	"github.com/gin-gonic/gin"
)

type AnimeHandler struct {
	animeSvc *animesvc.Service
	autoDL   *bangumisvc.AutoDownloader
}

func NewAnimeHandler(animeSvc *animesvc.Service, autoDL *bangumisvc.AutoDownloader) *AnimeHandler {
	return &AnimeHandler{animeSvc: animeSvc, autoDL: autoDL}
}

func (h *AnimeHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/anime")
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.POST("/", h.Create)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.GET("/:id/episodes", h.ListEpisodes)
	g.POST("/:id/episodes", h.CreateEpisode)
	g.DELETE("/:id/episodes/:episodeId", h.DeleteEpisode)
	g.POST("/parse_title", h.ParseTitle)
	g.POST("/:id/subscribe", h.Subscribe)
	g.POST("/:id/unsubscribe", h.Unsubscribe)
	g.GET("/:id/downloads", h.GetDownloads)
	g.PUT("/:id/offset", h.UpdateOffset)
	g.PUT("/:id/stream-preference", h.StreamPreference)
	g.POST("/:id/check-updates", h.CheckUpdates)
	g.POST("/check-all-updates", h.CheckAllUpdates)
	g.POST("/:id/collect", h.CollectSeason)
	g.POST("/:id/refresh_tmdb", h.RefreshTMDB)
}

func (h *AnimeHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	status := c.Query("status")
	subscribed := c.Query("subscribed") == "true"

	animes, total, err := h.animeSvc.List(c.Request.Context(), status, subscribed, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取番剧列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": animes, "total": total})
}

func (h *AnimeHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	anime, err := h.animeSvc.Get(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, anime)
}

func (h *AnimeHandler) Create(c *gin.Context) {
	var req model.AnimeCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	anime := &model.Anime{
		Title:         req.Title,
		OriginalTitle: req.OriginalTitle,
		Aliases:       req.Aliases,
		Description:   req.Description,
		Status:        req.Status,
		Season:        req.Season,
		Year:          req.Year,
		CoverURL:      req.CoverURL,
		EpisodeCount:  req.EpisodeCount,
		Directory:     req.Directory,
		OfficialTitle: req.OfficialTitle,
		TitleRaw:      req.TitleRaw,
		SeasonRaw:     req.SeasonRaw,
		GroupName:     req.GroupName,
		DPI:           req.DPI,
		Source:        req.Source,
		Subtitle:      req.Subtitle,
		EpsCollect:    req.EpsCollect,
		EpisodeOffset: req.EpisodeOffset,
		SeasonOffset:  req.SeasonOffset,
		Filter:        req.Filter,
		RSSLink:       req.RSSLink,
		AirWeekday:    req.AirWeekday,
		BangumiID:     req.BangumiID,
		BangumiRating: req.BangumiRating,
		IsSubscribed:  req.IsSubscribed,
	}

	if err := h.animeSvc.Create(c.Request.Context(), anime); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建番剧失败"})
		return
	}
	c.JSON(http.StatusCreated, anime)
}

func (h *AnimeHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	anime, err := h.animeSvc.Update(c.Request.Context(), uint(id), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新番剧失败"})
		return
	}
	c.JSON(http.StatusOK, anime)
}

func (h *AnimeHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.animeSvc.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除番剧失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "番剧已删除"})
}

func (h *AnimeHandler) ListEpisodes(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	episodes, err := h.animeSvc.ListEpisodes(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, episodes)
}

func (h *AnimeHandler) CreateEpisode(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var ep model.AnimeEpisode
	if err := c.ShouldBindJSON(&ep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.animeSvc.CreateEpisode(c.Request.Context(), uint(id), &ep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建集数失败"})
		return
	}
	c.JSON(http.StatusCreated, ep)
}

func (h *AnimeHandler) DeleteEpisode(c *gin.Context) {
	animeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	episodeID, _ := strconv.ParseUint(c.Param("episodeId"), 10, 64)
	if err := h.animeSvc.DeleteEpisode(c.Request.Context(), uint(animeID), uint(episodeID)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集数不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "集数已删除"})
}

func (h *AnimeHandler) ParseTitle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func (h *AnimeHandler) Subscribe(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	anime, err := h.animeSvc.Subscribe(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// 异步触发自动下载
	if h.autoDL != nil {
		go h.autoDL.TriggerAutoDownload(context.Background(), anime.ID, anime.Title)
	}
	c.JSON(http.StatusOK, gin.H{"detail": "订阅成功", "anime_id": anime.ID, "is_subscribed": true})
}

func (h *AnimeHandler) Unsubscribe(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	anime, err := h.animeSvc.Unsubscribe(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "取消订阅成功", "anime_id": anime.ID, "is_subscribed": false})
}

func (h *AnimeHandler) GetDownloads(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	downloads, err := h.animeSvc.GetDownloads(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, downloads)
}

func (h *AnimeHandler) UpdateOffset(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		EpisodeOffset *int `json:"episode_offset"`
		SeasonOffset  *int `json:"season_offset"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	anime, err := h.animeSvc.UpdateOffset(c.Request.Context(), uint(id), req.EpisodeOffset, req.SeasonOffset)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, anime)
}

func (h *AnimeHandler) StreamPreference(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		RuleID    uint   `json:"rule_id" binding:"required"`
		DetailURL string `json:"detail_url" binding:"required"`
		RoadName  string `json:"road_name"`
		RuleName  string `json:"rule_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	updates := map[string]interface{}{
		"stream_rule_id":     req.RuleID,
		"stream_detail_url":  req.DetailURL,
		"stream_road_name":   req.RoadName,
		"stream_rule_name":   req.RuleName,
		// 切换源时清空健康状态，等健康检测 job 重新评估
		"source_health_status": "",
		"source_health_note":   "",
	}

	anime, err := h.animeSvc.Update(c.Request.Context(), uint(id), updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存源偏好失败"})
		return
	}
	c.JSON(http.StatusOK, anime)
}

func (h *AnimeHandler) CheckUpdates(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if h.autoDL == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "自动下载服务不可用"})
		return
	}

	// 获取 anime
	anime, err := h.animeSvc.Get(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "番剧不存在"})
		return
	}
	if !anime.IsSubscribed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未追番的番剧不能检查更新"})
		return
	}

	go h.autoDL.TriggerManualCheck(context.Background(), anime.ID, anime.Title)
	c.JSON(http.StatusAccepted, gin.H{"detail": "检查更新任务已提交"})
}

func (h *AnimeHandler) CheckAllUpdates(c *gin.Context) {
	if h.autoDL == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "自动下载服务不可用"})
		return
	}
	go h.autoDL.CheckAllSubscribed(context.Background())
	c.JSON(http.StatusAccepted, gin.H{"detail": "全量检查任务已提交"})
}

func (h *AnimeHandler) CollectSeason(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	c.JSON(http.StatusOK, gin.H{"detail": "整季收集功能开发中", "anime_id": id})
}

func (h *AnimeHandler) RefreshTMDB(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	c.JSON(http.StatusOK, gin.H{"detail": "TMDB 刷新功能开发中", "anime_id": id})
}
