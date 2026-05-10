package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/orchestrator"
	"github.com/anidog/anidog-go/internal/service/setting"
)

// OrchestratorHandler 暴露 Orchestrator 相关接口（诊断 + 手动触发）。
type OrchestratorHandler struct {
	db         *gorm.DB
	orch       *orchestrator.Orchestrator
	settingSvc *setting.Service
}

func NewOrchestratorHandler(db *gorm.DB, orch *orchestrator.Orchestrator, settingSvc *setting.Service) *OrchestratorHandler {
	return &OrchestratorHandler{db: db, orch: orch, settingSvc: settingSvc}
}

func (h *OrchestratorHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/anime/:id/diagnosis", h.Diagnosis)
	rg.POST("/anime/:id/orchestrate", h.RunOne)
	rg.POST("/orchestrator/run-all", h.RunAll)
}

// Diagnosis GET /anime/:id/diagnosis
// 返回该番剧所有集的最新诊断记录。
func (h *OrchestratorHandler) Diagnosis(c *gin.Context) {
	animeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if animeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 anime id"})
		return
	}

	var records []model.OrchestratorDiagnosis
	h.db.WithContext(c.Request.Context()).
		Where("anime_id = ?", animeID).
		Order("episode_number ASC, checked_at DESC").
		Find(&records)

	// 按 (episode, source) 分组输出
	type epSource struct {
		SourceType string                        `json:"source_type"`
		CheckedAt  string                        `json:"checked_at"`
		ResultCount int                          `json:"result_count"`
		RankedOut  int                           `json:"ranked_out"`
		Reason     string                        `json:"reason"`
		BestTitle  string                        `json:"best_title,omitempty"`
		BestScore  float64                       `json:"best_score,omitempty"`
	}
	type epEntry struct {
		EpisodeNumber int                 `json:"episode_number"`
		Sources       map[string]epSource `json:"sources"`
	}
	grouped := map[int]*epEntry{}
	for _, r := range records {
		e, ok := grouped[r.EpisodeNumber]
		if !ok {
			e = &epEntry{EpisodeNumber: r.EpisodeNumber, Sources: map[string]epSource{}}
			grouped[r.EpisodeNumber] = e
		}
		// 同一 (ep, source) 只保留最新一条（记录顺序已 DESC）
		if _, exists := e.Sources[r.SourceType]; exists {
			continue
		}
		e.Sources[r.SourceType] = epSource{
			SourceType:  r.SourceType,
			CheckedAt:   r.CheckedAt.Format("2006-01-02 15:04:05"),
			ResultCount: r.ResultCount,
			RankedOut:   r.RankedOut,
			Reason:      r.Reason,
			BestTitle:   r.BestTitle,
			BestScore:   r.BestScore,
		}
	}
	out := make([]epEntry, 0, len(grouped))
	for _, e := range grouped {
		out = append(out, *e)
	}

	c.JSON(http.StatusOK, gin.H{"episodes": out})
}

// RunOne POST /anime/:id/orchestrate
// 触发 Orchestrator 对单个番剧跑一轮（异步执行）
func (h *OrchestratorHandler) RunOne(c *gin.Context) {
	animeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if animeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 anime id"})
		return
	}

	var anime model.Anime
	if err := h.db.First(&anime, animeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "番剧不存在"})
		return
	}

	go func() {
		ctx := context.Background()
		global := orchestrator.LoadGlobal(ctx, h.settingSvc)
		h.orch.CheckAnime(ctx, &anime, global)
	}()

	c.JSON(http.StatusAccepted, gin.H{"detail": "已触发检查"})
}

// RunAll POST /orchestrator/run-all
// 触发 Orchestrator 全量检查（异步）
func (h *OrchestratorHandler) RunAll(c *gin.Context) {
	go func() {
		h.orch.CheckAllSubscribed(context.Background())
	}()
	c.JSON(http.StatusAccepted, gin.H{"detail": "已触发全量检查"})
}
