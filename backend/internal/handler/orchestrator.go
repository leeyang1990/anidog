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
		SourceType  string  `json:"source_type"`
		CheckedAt   string  `json:"checked_at"`
		ResultCount int     `json:"result_count"`
		RankedOut   int     `json:"ranked_out"`
		Reason      string  `json:"reason"`
		BestTitle   string  `json:"best_title,omitempty"`
		BestScore   float64 `json:"best_score,omitempty"`
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
		h.orch.CheckAnimeWithSettings(ctx, &anime)
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
//   - completed: 来自逐集归档记录或单集 download；合集范围不代表全部已归档
//   - downloading / pending / paused: 来自单集任务或覆盖该集的合集任务
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
	maxEp := expected
	for ep := range epMeta {
		if ep > maxEp {
			maxEp = ep
		}
	}

	// 2. download 表里有效记录（按集号最新一条优先级：completed > downloading > pending > failed）
	var downloads []model.Download
	h.db.WithContext(c.Request.Context()).
		Where("anime_id = ?", animeID).
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
	}
	dlByEp := make(map[int]dlInfo, len(downloads))
	dlByTorrentID := make(map[string]model.Download, len(downloads))
	addDownload := func(ep int, d model.Download) {
		if ep <= 0 {
			return
		}
		old, has := dlByEp[ep]
		if !has || statusRank[d.Status] > statusRank[old.Status] {
			dlByEp[ep] = dlInfo{Status: d.Status, Source: d.Source, ID: d.ID}
		}
	}
	for _, d := range downloads {
		dlByTorrentID[d.TorrentID] = d
		if _, valid := statusRank[d.Status]; !valid || (d.Status == model.DownloadStatusCompleted && d.MediaMissing) {
			continue
		}
		if d.Scope == model.DownloadScopeSeason {
			// Completed packs may have partial coverage. Only animeepisode can
			// establish which files were actually archived, including EP1.
			if d.Status == model.DownloadStatusCompleted || d.EpisodeStart == nil || d.EpisodeEnd == nil || *d.EpisodeStart <= 0 || *d.EpisodeEnd < *d.EpisodeStart {
				continue
			}
			for ep := *d.EpisodeStart; ep <= *d.EpisodeEnd && ep <= maxEp; ep++ {
				addDownload(ep, d)
			}
			continue
		}
		if d.EpisodeNumber == nil {
			continue
		}
		addDownload(*d.EpisodeNumber, d)
	}
	for _, meta := range episodes {
		if !meta.Downloaded {
			continue
		}
		archived := dlInfo{Status: model.DownloadStatusCompleted}
		if meta.DownloadID != nil {
			if d, ok := dlByTorrentID[*meta.DownloadID]; ok {
				if d.MediaMissing {
					continue
				}
				archived.Source, archived.ID = d.Source, d.ID
			}
		}
		dlByEp[meta.EpisodeNumber] = archived
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
	var upgrade *model.Download
	for i := range downloads {
		d := &downloads[i]
		if d.Purpose == model.DownloadPurposeSeasonUpgrade && (d.Status == model.DownloadStatusPending || d.Status == model.DownloadStatusDownloading || d.Status == model.DownloadStatusPaused) {
			upgrade = d
			break
		}
	}
	type epStatus struct {
		QualityStatus     string      `json:"quality_status,omitempty"`
		QualityReason     string      `json:"quality_reason,omitempty"`
		UpgradeDownloadID uint        `json:"upgrade_download_id,omitempty"`
		UpgradeStatus     string      `json:"upgrade_status,omitempty"`
		EpisodeNumber     int         `json:"episode_number"`
		Status            string      `json:"status"` // completed/downloading/pending/upcoming/no_resource/missing
		Source            string      `json:"source,omitempty"`
		DownloadID        uint        `json:"download_id,omitempty"`
		FilePath          *string     `json:"file_path,omitempty"`
		FileSize          *int64      `json:"file_size,omitempty"`
		AirDate           string      `json:"air_date,omitempty"`
		NameCN            string      `json:"name_cn,omitempty"`
		Title             string      `json:"title,omitempty"`
		IsAired           bool        `json:"is_aired"`
		Diagnosis         []diagBrief `json:"diagnosis,omitempty"`
	}

	out := make([]epStatus, 0, expected)
	for ep := 1; ep <= maxEp; ep++ {
		row := epStatus{EpisodeNumber: ep, IsAired: true}
		if upgrade != nil && upgrade.EpisodeStart != nil && upgrade.EpisodeEnd != nil && ep >= *upgrade.EpisodeStart && ep <= *upgrade.EpisodeEnd {
			row.UpgradeDownloadID = upgrade.ID
			row.UpgradeStatus = upgrade.Status
		}
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

		if dl, has := dlByEp[ep]; has {
			row.Status = dl.Status
			row.Source = dl.Source
			row.DownloadID = dl.ID
			if dl.Status == model.DownloadStatusCompleted {
				row.FilePath, row.FileSize = epMeta[ep].FilePath, epMeta[ep].FileSize
				row.QualityStatus, row.QualityReason = epMeta[ep].QualityStatus, epMeta[ep].QualityReason
				if row.QualityStatus == "" {
					row.QualityStatus = model.QualityUnknown
				}
			}
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

	summary := gin.H{"available": 0, "meets_target": 0, "upgrade_needed": 0, "unknown": 0}
	for _, row := range out {
		if row.Status == model.DownloadStatusCompleted {
			summary["available"] = summary["available"].(int) + 1
			key := row.QualityStatus
			if key == "" {
				key = model.QualityUnknown
			}
			if n, ok := summary[key].(int); ok {
				summary[key] = n + 1
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"quality_summary": summary,
		"season_finished": episode.SeasonFinished(&anime, episodes, now),
		"season_upgrade":  upgrade,
		"anime_id":        anime.ID,
		"title":           anime.Title,
		"episode_count":   expected,
		"episodes":        out,
	})
}
