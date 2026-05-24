package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/episode"
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
	rg.GET("/anime/:id/episode-status", h.EpisodeStatus)
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

// EpisodeStatus GET /anime/:id/episode-status
// 返回该番剧每一集的综合状态：
//   - completed / downloading / pending: 来自 download 表
//   - upcoming: 来自 animeepisode.air_date 在未来
//   - missing: 既未下载也无诊断（可能尚未扫到）
//   - no_resource: 既未下载，且 orchestrator_diagnosis 有最新失败记录
//
// 同时返回 air_date / name_cn 让前端展示"待发布（X月X日）"。
func (h *OrchestratorHandler) EpisodeStatus(c *gin.Context) {
	animeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if animeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 anime id"})
		return
	}

	var anime model.Anime
	if err := h.db.WithContext(c.Request.Context()).First(&anime, animeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "番剧不存在"})
		return
	}

	expected := 0
	if anime.EpisodeCount != nil {
		expected = *anime.EpisodeCount
	}

	// 1. animeepisode 元数据（air_date / name_cn）
	var episodes []model.AnimeEpisode
	h.db.WithContext(c.Request.Context()).
		Where("anime_id = ?", animeID).
		Order("episode_number ASC").
		Find(&episodes)
	epMeta := make(map[int]model.AnimeEpisode, len(episodes))
	for _, e := range episodes {
		epMeta[e.EpisodeNumber] = e
	}

	// 2. download 表里有效记录（按集号最新一条优先级：completed > downloading > pending > failed）
	var downloads []model.Download
	h.db.WithContext(c.Request.Context()).
		Where("anime_id = ? AND episode_number IS NOT NULL", animeID).
		Order("created_at DESC").
		Find(&downloads)
	type dlInfo struct {
		Status string
		Source string
		ID     uint
	}
	statusRank := map[string]int{
		model.DownloadStatusCompleted:   4,
		model.DownloadStatusDownloading: 3,
		model.DownloadStatusPending:     2,
		model.DownloadStatusPaused:      1,
		model.DownloadStatusFailed:      0,
	}
	dlByEp := make(map[int]dlInfo, len(downloads))
	for _, d := range downloads {
		if d.EpisodeNumber == nil {
			continue
		}
		ep := *d.EpisodeNumber
		old, has := dlByEp[ep]
		if !has || statusRank[d.Status] > statusRank[old.Status] {
			dlByEp[ep] = dlInfo{Status: d.Status, Source: d.Source, ID: d.ID}
		}
	}

	// 3. 诊断记录（最新失败原因）
	var diags []model.OrchestratorDiagnosis
	h.db.WithContext(c.Request.Context()).
		Where("anime_id = ?", animeID).
		Order("checked_at DESC").
		Find(&diags)
	type diagBrief struct {
		SourceType string `json:"source_type"`
		Reason     string `json:"reason"`
	}
	diagByEp := make(map[int][]diagBrief, len(diags))
	seenSource := make(map[string]bool) // ep#src 去重
	for _, d := range diags {
		key := strconv.Itoa(d.EpisodeNumber) + "#" + d.SourceType
		if seenSource[key] {
			continue
		}
		seenSource[key] = true
		diagByEp[d.EpisodeNumber] = append(diagByEp[d.EpisodeNumber], diagBrief{
			SourceType: d.SourceType,
			Reason:     d.Reason,
		})
	}

	now := time.Now()
	type epStatus struct {
		EpisodeNumber int         `json:"episode_number"`
		Status        string      `json:"status"` // completed/downloading/pending/upcoming/no_resource/missing
		Source        string      `json:"source,omitempty"`
		DownloadID    uint        `json:"download_id,omitempty"`
		AirDate       string      `json:"air_date,omitempty"`
		NameCN        string      `json:"name_cn,omitempty"`
		Title         string      `json:"title,omitempty"`
		IsAired       bool        `json:"is_aired"`
		Diagnosis     []diagBrief `json:"diagnosis,omitempty"`
	}

	out := make([]epStatus, 0, expected)
	maxEp := expected
	// 兜底：如果 episodeCount 没设置，至少把 animeepisode 表里看到的都返回
	for ep := range epMeta {
		if ep > maxEp {
			maxEp = ep
		}
	}
	for ep := 1; ep <= maxEp; ep++ {
		row := epStatus{EpisodeNumber: ep, IsAired: true}
		if meta, has := epMeta[ep]; has {
			if meta.AirDate != nil {
				row.AirDate = *meta.AirDate
				row.IsAired = episode.IsAired(*meta.AirDate, now)
			}
			if meta.NameCN != nil {
				row.NameCN = *meta.NameCN
			}
			if meta.Title != nil {
				row.Title = *meta.Title
			}
		}

		if dl, has := dlByEp[ep]; has && dl.Status != model.DownloadStatusFailed {
			row.Status = dl.Status
			row.Source = dl.Source
			row.DownloadID = dl.ID
		} else if !row.IsAired {
			row.Status = "upcoming"
		} else if diag, has := diagByEp[ep]; has {
			row.Status = "no_resource"
			row.Diagnosis = diag
		} else {
			row.Status = "missing"
		}

		out = append(out, row)
	}

	c.JSON(http.StatusOK, gin.H{
		"anime_id":      anime.ID,
		"title":         anime.Title,
		"episode_count": expected,
		"episodes":      out,
	})
}
