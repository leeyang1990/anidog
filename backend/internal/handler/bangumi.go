package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	bangumisvc "github.com/anidog/anidog-go/internal/service/bangumi"
	"github.com/anidog/anidog-go/internal/service"
)

type BangumiHandler struct {
	animeSvc   *animesvc.Service
	bangumiSvc *service.BangumiService
	autoDL     *bangumisvc.AutoDownloader
}

func NewBangumiHandler(animeSvc *animesvc.Service, bangumiSvc *service.BangumiService, autoDL *bangumisvc.AutoDownloader) *BangumiHandler {
	return &BangumiHandler{animeSvc: animeSvc, bangumiSvc: bangumiSvc, autoDL: autoDL}
}

func (h *BangumiHandler) RegisterRoutes(rg *gin.RouterGroup) {
	bangumi := rg.Group("/bangumi")
	bangumi.GET("/search", h.Search)
	bangumi.POST("/discover", h.Discover)
	bangumi.GET("/trending", h.Trending)
	bangumi.GET("/calendar", h.Calendar)
	bangumi.GET("/:id", h.GetDetail)
	bangumi.GET("/:id/characters", h.GetCharacters)
	bangumi.GET("/characters/:id", h.GetCharacterDetail)
	bangumi.POST("/:id/subscribe", h.Subscribe)
	bangumi.DELETE("/:id/subscribe", h.Unsubscribe)
}

// Trending Bangumi 热门趋势（Kazumi 首页同款）
func (h *BangumiHandler) Trending(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	results, total, err := h.bangumiSvc.GetTrending(c.Request.Context(), limit, offset)
	if err != nil {
		zap.L().Error("获取热门趋势失败", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"total": 0, "results": []interface{}{}})
		return
	}

	subMap := h.animeSvc.GetSubscriptionMap(c.Request.Context())
	var output []model.BangumiAnimeWithStatus
	for _, item := range results {
		entry := model.BangumiAnimeWithStatus{BangumiAnime: item}
		if sub, ok := subMap[item.ID]; ok {
			entry.IsSubscribed = sub.IsSubscribed
			entry.LocalID = sub.LocalID
		}
		output = append(output, entry)
	}

	c.JSON(http.StatusOK, gin.H{"total": total, "results": output})
}

// Discover 多维度番剧发现：keyword + year+seasons + tags + sort + min_rating
func (h *BangumiHandler) Discover(c *gin.Context) {
	var req struct {
		Keyword   string   `json:"keyword"`
		Sort      string   `json:"sort"`
		Year      int      `json:"year"`
		Season    string   `json:"season"`   // 兼容旧字段
		Seasons   []string `json:"seasons"`  // 新字段：多选
		Tags      []string `json:"tags"`
		MinRating float64  `json:"min_rating"`
		Limit     int      `json:"limit"`
		Offset    int      `json:"offset"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "参数错误: " + err.Error()})
		return
	}

	// 兼容：如果传了单个 season，合入 seasons 数组
	seasons := req.Seasons
	if req.Season != "" && len(seasons) == 0 {
		seasons = []string{req.Season}
	}

	opts := &service.DiscoverOptions{
		Keyword:   req.Keyword,
		Sort:      req.Sort,
		Year:      req.Year,
		Season:    req.Season, // 兼容
		Seasons:   seasons,
		Tags:      req.Tags,
		MinRating: req.MinRating,
		Limit:     req.Limit,
		Offset:    req.Offset,
	}
	results, total, err := h.bangumiSvc.Discover(c.Request.Context(), opts)
	if err != nil {
		zap.L().Error("Bangumi 发现失败", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"total": 0, "results": []interface{}{}})
		return
	}

	subMap := h.animeSvc.GetSubscriptionMap(c.Request.Context())
	var output []model.BangumiAnimeWithStatus
	for _, item := range results {
		entry := model.BangumiAnimeWithStatus{BangumiAnime: item}
		if sub, ok := subMap[item.ID]; ok {
			entry.IsSubscribed = sub.IsSubscribed
			entry.LocalID = sub.LocalID
		}
		output = append(output, entry)
	}

	c.JSON(http.StatusOK, gin.H{"total": total, "results": output})
}

func (h *BangumiHandler) Search(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "keyword 参数不能为空"})
		return
	}

	results, err := h.bangumiSvc.SearchAnime(c.Request.Context(), keyword)
	if err != nil {
		zap.L().Error("Bangumi 搜索失败", zap.String("keyword", keyword), zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"keyword": keyword, "results": []interface{}{}})
		return
	}

	subMap := h.animeSvc.GetSubscriptionMap(c.Request.Context())

	var output []model.BangumiAnimeWithStatus
	for _, item := range results {
		entry := model.BangumiAnimeWithStatus{BangumiAnime: item}
		if sub, ok := subMap[item.ID]; ok {
			entry.IsSubscribed = sub.IsSubscribed
			entry.LocalID = sub.LocalID
		}
		output = append(output, entry)
	}

	c.JSON(http.StatusOK, gin.H{"keyword": keyword, "results": output})
}

func (h *BangumiHandler) Calendar(c *gin.Context) {
	calendar, err := h.bangumiSvc.GetCalendar(c.Request.Context())
	if err != nil {
		zap.L().Error("获取 Bangumi 日历失败", zap.Error(err))
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	subMap := h.animeSvc.GetSubscriptionMap(c.Request.Context())

	var output []model.BangumiCalendarDayWithStatus
	for _, day := range calendar {
		d := model.BangumiCalendarDayWithStatus{WeekdayID: day.WeekdayID, WeekdayCN: day.WeekdayCN}
		for _, item := range day.Items {
			entry := model.BangumiAnimeWithStatus{BangumiAnime: item}
			if sub, ok := subMap[item.ID]; ok {
				entry.IsSubscribed = sub.IsSubscribed
				entry.LocalID = sub.LocalID
			}
			d.Items = append(d.Items, entry)
		}
		output = append(output, d)
	}

	c.JSON(http.StatusOK, output)
}

func (h *BangumiHandler) GetDetail(c *gin.Context) {
	bangumiID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 Bangumi ID"})
		return
	}

	detail, err := h.bangumiSvc.GetAnimeDetail(c.Request.Context(), bangumiID)
	if err != nil {
		zap.L().Error("获取 Bangumi 详情失败", zap.Int("bangumi_id", bangumiID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"detail": "获取番剧详情失败: " + err.Error()})
		return
	}

	result := model.BangumiAnimeWithStatus{BangumiAnime: *detail}
	if local, err := h.animeSvc.FindByBangumiID(c.Request.Context(), bangumiID); err == nil {
		result.IsSubscribed = local.IsSubscribed
		result.LocalID = local.ID
	}

	c.JSON(http.StatusOK, result)
}

// GetCharacters 获取番剧角色与声优
func (h *BangumiHandler) GetCharacters(c *gin.Context) {
	bangumiID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 Bangumi ID"})
		return
	}
	chars, err := h.bangumiSvc.GetCharacters(c.Request.Context(), bangumiID)
	if err != nil {
		zap.L().Warn("获取角色失败", zap.Error(err))
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	c.JSON(http.StatusOK, chars)
}

// GetCharacterDetail 获取角色详情（/bangumi/characters/:id）
func (h *BangumiHandler) GetCharacterDetail(c *gin.Context) {
	charID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的角色 ID"})
		return
	}
	detail, err := h.bangumiSvc.GetCharacterDetail(c.Request.Context(), charID)
	if err != nil {
		zap.L().Warn("获取角色详情失败", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"detail": "角色不存在"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *BangumiHandler) Subscribe(c *gin.Context) {
	bangumiID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 Bangumi ID"})
		return
	}

	detail, err := h.bangumiSvc.GetAnimeDetail(c.Request.Context(), bangumiID)
	if err != nil {
		zap.L().Warn("获取 Bangumi 详情失败", zap.Int("bangumi_id", bangumiID), zap.Error(err))
	}

	local, err := h.animeSvc.FindByBangumiID(c.Request.Context(), bangumiID)
	zap.L().Debug("Bangumi 订阅查询结果",
		zap.Int("bangumi_id", bangumiID),
		zap.Any("detail", detail),
		zap.Any("local_err", err),
		zap.Any("local_anime", local))

	if animesvc.IsNotFound(err) {
		local, err = h.animeSvc.CreateFromBangumi(c.Request.Context(), bangumiID, detail)
		if err != nil {
			zap.L().Error("创建番剧记录失败", zap.Int("bangumi_id", bangumiID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建番剧记录失败"})
			return
		}
		zap.L().Info("通过 Bangumi 订阅创建新番剧", zap.Int("bangumi_id", bangumiID), zap.Uint("anime_id", local.ID))
	} else if err != nil {
		zap.L().Error("查询番剧记录失败", zap.Int("bangumi_id", bangumiID))
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "查询番剧记录失败"})
		return
	} else {
		if !local.IsSubscribed {
			h.animeSvc.Subscribe(c.Request.Context(), local.ID)
			zap.L().Info("更新番剧订阅状态", zap.Uint("anime_id", local.ID))
		}
		if detail != nil {
			h.animeSvc.RefreshFromBangumi(c.Request.Context(), local.ID, detail)
		}
	}

	go h.autoDL.TriggerAutoDownload(context.Background(), local.ID, local.Title)

	c.JSON(http.StatusOK, gin.H{
		"detail":   "订阅成功",
		"anime_id": local.ID,
		"title":    local.Title,
	})
}

func (h *BangumiHandler) Unsubscribe(c *gin.Context) {
	bangumiID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 Bangumi ID"})
		return
	}

	local, err := h.animeSvc.FindByBangumiID(c.Request.Context(), bangumiID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "未找到对应 Bangumi ID 的番剧"})
		return
	}

	if local.IsSubscribed {
		h.animeSvc.Unsubscribe(c.Request.Context(), local.ID)
		zap.L().Info("取消番剧订阅", zap.Int("bangumi_id", bangumiID), zap.Uint("anime_id", local.ID))
	}

	c.JSON(http.StatusOK, gin.H{"detail": "取消订阅成功"})
}
