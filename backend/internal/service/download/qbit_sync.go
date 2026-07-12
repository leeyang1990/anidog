package download

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/notification"
)

// QBitSyncer 从 qBit 同步 BT 下载的真实进度/大小到 DB。
// 设计为 scheduler.Job，可定时调用。
type QBitSyncer struct {
	db       *gorm.DB
	baseURL  string
	user     string
	pass     string
	client   *http.Client
	notifSvc *notification.Service // 可空：未配置时不发通知
}

func NewQBitSyncer(db *gorm.DB, cfg *config.Config) *QBitSyncer {
	jar, _ := cookiejar.New(nil)
	return &QBitSyncer{
		db:      db,
		baseURL: strings.TrimSuffix(cfg.DownloaderHost, "/"),
		user:    cfg.DownloaderUsername,
		pass:    cfg.DownloaderPassword,
		client:  &http.Client{Jar: jar, Timeout: 15 * time.Second},
	}
}

// SetNotificationService 注入通知服务（main.go 在 wiring 阶段调用）。
// 拆出来是因为 notification 依赖 model 而 download 也依赖 model，
// 走 setter 注入避免构造函数循环膨胀，未注入时 notify() 直接 no-op。
func (s *QBitSyncer) SetNotificationService(n *notification.Service) {
	s.notifSvc = n
}

func (s *QBitSyncer) Name() string { return "qbit_sync" }

// Run 实现 scheduler.Job 接口。
func (s *QBitSyncer) Run(ctx context.Context) {
	if err := s.Sync(ctx); err != nil {
		zap.L().Warn("qBit 同步失败", zap.Error(err))
	}
}

// Sync 拉取 qBit 所有种子信息，按 info_hash 更新对应 Download 记录。
func (s *QBitSyncer) Sync(ctx context.Context) error {
	if err := s.ensureLogin(ctx); err != nil {
		return err
	}

	torrents, err := s.listTorrents(ctx)
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		return nil
	}

	// 按 info_hash 做映射
	byHash := make(map[string]map[string]interface{}, len(torrents))
	for _, t := range torrents {
		if h, ok := t["hash"].(string); ok && h != "" {
			byHash[strings.ToUpper(h)] = t
		}
	}

	// 查 DB 里所有有 info_hash 的 BT 下载
	var downloads []model.Download
	if err := s.db.WithContext(ctx).
		Where("download_type = ? AND info_hash IS NOT NULL", model.DownloadTypeTorrent).
		Find(&downloads).Error; err != nil {
		return err
	}

	updated := 0
	orphaned := 0
	abandoned := 0
	for _, dl := range downloads {
		if dl.InfoHash == nil {
			continue
		}
		qt, ok := byHash[strings.ToUpper(*dl.InfoHash)]
		if !ok {
			// qBit 里找不到这个 hash —— 大概率是 qBit 容器重启/数据丢失。
			// 仅把"还在进行中"的状态翻成 failed，让 Orchestrator 下一轮重新挑源。
			// 已 completed / failed / paused 的不动。
			// 保护：刚创建 < 60s 的任务给 qBit 一点时间索引，跳过；
			// >= 60s 还找不到才认定为孤儿。
			if (dl.Status == model.DownloadStatusDownloading ||
				dl.Status == model.DownloadStatusPending) &&
				time.Since(dl.CreatedAt) > time.Minute {
				orphanErr := fmt.Errorf("qBittorrent 中不存在对应任务，可能已被删除或下载器状态丢失")
				kind, delay := classifyError(orphanErr, dl.RetryCount)
				updates := map[string]interface{}{
					"status":         model.DownloadStatusFailed,
					"download_speed": 0,
					"eta":            nil,
					"failure_kind":   kind,
					"last_error":     orphanErr.Error(),
				}
				if delay > 0 {
					nextAt := time.Now().Add(delay)
					updates["next_retry_at"] = &nextAt
				} else {
					updates["next_retry_at"] = nil
				}
				if err := s.db.Model(&model.Download{}).
					Where("id = ?", dl.ID).
					Updates(updates).Error; err != nil {
					zap.L().Warn("标记孤儿下载为 failed 失败",
						zap.Uint("id", dl.ID), zap.Error(err))
					continue
				}
				orphaned++
				zap.L().Info("下载任务在 qBit 中已不存在，置为 failed",
					zap.Uint("id", dl.ID),
					zap.String("info_hash", *dl.InfoHash),
					zap.String("name", dl.Name))
			}
			continue
		}
		updates := map[string]interface{}{}
		if v, ok := qt["size"].(float64); ok && v > 0 {
			size := int64(v)
			updates["total_bytes"] = size
		}
		if v, ok := qt["downloaded"].(float64); ok {
			updates["downloaded_bytes"] = int64(v)
		}
		if v, ok := qt["progress"].(float64); ok {
			updates["progress"] = v * 100.0
		}
		if v, ok := qt["dlspeed"].(float64); ok {
			updates["download_speed"] = int64(v)
		}
		if v, ok := qt["eta"].(float64); ok && v > 0 && v < 8640000 {
			eta := int(v)
			updates["eta"] = eta
		}
		// 状态映射
		if state, ok := qt["state"].(string); ok {
			newStatus := mapQBitState(state)
			// 防误标：刚入队 < 60s 内不允许翻 completed。qBit 在创建种子的早期
			// 阶段会瞬时上报 checkingUP/queuedUP/forcedUP 等状态，mapQBitState
			// 会把它们映射为 completed —— 但此时种子可能还在等元数据，并没有真下完。
			// 等过 60s 再让 mapQBitState 决定，能消除"种子刚入队就发 S0xExx 已更新"的误推。
			fakeCompleted := newStatus == model.DownloadStatusCompleted &&
				time.Since(dl.CreatedAt) < 60*time.Second
			if fakeCompleted {
				zap.L().Debug("qbit_sync: 入队 < 60s 期间忽略 completed 状态映射",
					zap.Uint("id", dl.ID),
					zap.String("qbit_state", state))
			}
			if newStatus != "" && newStatus != dl.Status && !fakeCompleted {
				updates["status"] = newStatus
				if newStatus == model.DownloadStatusCompleted && dl.CompletedAt == nil {
					now := time.Now()
					updates["completed_at"] = &now
					// 状态从非 completed 翻成 completed，且本次同步前还没标完成 →
					// 是这一轮新发现的"完成事件"，触发通知。
					// 放在循环里发是 fire-and-forget，不阻塞同步主流程。
					s.notifyCompletion(ctx, &dl)
				}
			}
			// 死种快速放弃 —— 分三档：
			//
			//  A. metaDL 卡住 ≥ 90s：DHT 找不到任何 peer，连元数据都拉不到，是死种最强信号。
			//     正常种子 5-15s 内就会进入 stalledDL/downloading；超过 90s 还停在 metaDL
			//     基本就是 magnet 没活种 + DHT 网络也搜不到（即便 BT 端口暴露 6881）。
			//
			//  B. stalledDL + has_metadata + 进度 <1% + 0 seeders 持续 5min：
			//     swarm 里看似还有残骸（DHT 能 announce 出来一些 leechers），但没有任何完整源，
			//     拼不起来。典型现象：seen_complete 是任务刚加进去那几秒，之后就一直停在 0.x%。
			//     注入公共 tracker 后还救不活就放弃，让 orchestrator 换 mikan_rss 下一名次的种。
			//
			//  C. stalledDL/missingFiles 持续 6h 且 0 元数据 0 做种 0 进度：兜底。
			//     这一档专门收"漏掉的边角情况"——比如 has_metadata=false 但状态被映射成 stalledDL。
			//
			// 命中任一档：写黑名单 + 从 qBit 删除 + 从 DB 删除
			// （Orchestrator 下一轮就会从剩下的 mikan_rss 候选里挑下一名次的种）。
			if state == "metaDL" && time.Since(dl.CreatedAt) > 90*time.Second {
				if s.abandonDeadTorrent(ctx, &dl, "metaDL 超 90s 无元数据，DHT 找不到任何 peer，判死种") {
					abandoned++
				}
				continue
			}
			if state == "stalledDL" && time.Since(dl.CreatedAt) > 5*time.Minute {
				hasMeta, _ := qt["has_metadata"].(bool)
				numSeeds, _ := qt["num_seeds"].(float64)
				progress, _ := qt["progress"].(float64)
				dlspeed, _ := qt["dlspeed"].(float64)
				// has_metadata=true 的种子才适用这条快速判定（已拿到 .torrent 信息但找不到完整源）
				// progress < 0.01 即 < 1%；同时 num_seeds=0 且 dlspeed=0 → 5min 内未恢复
				if hasMeta && numSeeds <= 0 && dlspeed <= 0 && progress < 0.01 {
					if s.abandonDeadTorrent(ctx, &dl, "stalledDL 超 5min 无 seeder 无进度（swarm 里全是不完整副本），判死种") {
						abandoned++
					}
					continue
				}
			}
			if (state == "stalledDL" || state == "missingFiles") &&
				time.Since(dl.CreatedAt) > 6*time.Hour {
				numComplete, _ := qt["num_complete"].(float64)
				numSeeds, _ := qt["num_seeds"].(float64)
				dlspeed, _ := qt["dlspeed"].(float64)
				progress, _ := qt["progress"].(float64)
				if numComplete <= 0 && numSeeds <= 0 && dlspeed <= 0 && progress <= 0 {
					if s.abandonDeadTorrent(ctx, &dl, "qBit "+state+" 超 6h 仍 0 元数据 0 做种") {
						abandoned++
					}
					continue
				}
			}
		}

		if len(updates) > 0 {
			if err := s.db.Model(&model.Download{}).
				Where("id = ?", dl.ID).
				Updates(updates).Error; err != nil {
				zap.L().Warn("更新下载进度失败",
					zap.Uint("id", dl.ID), zap.Error(err))
				continue
			}
			updated++
		}
	}

	if updated > 0 || orphaned > 0 || abandoned > 0 {
		zap.L().Info("qBit 同步完成",
			zap.Int("matched", updated),
			zap.Int("orphaned_to_failed", orphaned),
			zap.Int("abandoned_dead_seed", abandoned),
			zap.Int("qbit_total", len(torrents)),
			zap.Int("db_total", len(downloads)))
	}
	return nil
}

// notifyCompletion 在某条 download 翻成 completed 时推一条通知。
//
// 设计：
//   - notifSvc 没注入 → 直接 no-op，不影响同步主流程
//   - 信息组装尽量"宽容"：anime 关联可能为空（手动下载没绑番），那时就用 dl.Name 当标题
//   - 异步发送：开 goroutine + 独立 timeout，避免 sync 主循环被 HTTP 拖慢
//   - 用独立 ctx：sync 主 ctx 一旦完成会被 cancel，会把还没发完的请求中断掉
func (s *QBitSyncer) notifyCompletion(ctx context.Context, dl *model.Download) {
	if s.notifSvc == nil {
		return
	}

	info := &notification.NotificationInfo{}

	// 优先用关联 anime 的 official_title / season，没绑番就用下载名兜底
	if dl.AnimeID != nil && *dl.AnimeID > 0 {
		var a model.Anime
		if err := s.db.WithContext(ctx).First(&a, *dl.AnimeID).Error; err == nil {
			if a.OfficialTitle != nil && *a.OfficialTitle != "" {
				info.OfficialTitle = *a.OfficialTitle
			} else {
				info.OfficialTitle = a.Title
			}
			if a.Season != nil {
				info.Season = *a.Season
			} else {
				info.Season = 1
			}
			if a.CoverURL != nil && *a.CoverURL != "" {
				info.CoverURL = *a.CoverURL
			}
		}
	}
	if info.OfficialTitle == "" {
		info.OfficialTitle = dl.Name
	}
	if info.Season == 0 {
		info.Season = 1
	}
	if dl.EpisodeNumber != nil {
		info.Episode = *dl.EpisodeNumber
	}

	// 幂等闸门：见 notification_dedup.go
	animeID := uint(0)
	if dl.AnimeID != nil {
		animeID = *dl.AnimeID
	}
	if !claimEpisodeNotification(s.db, animeID, info.Episode, info.Season) {
		return
	}

	go func(info *notification.NotificationInfo) {
		// 给所有渠道总共 30s 时间发完
		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.notifSvc.Broadcast(sendCtx, info)
	}(info)
}

// abandonDeadTorrent 永久放弃一个死种：
//  1. qBit 端删除（连同已下载的部分文件）
//  2. download 表删除该行（避免下载列表里堆积"失败"幽灵任务）
//  3. abandoned_torrent 表写一行黑名单，Orchestrator 后续不再重抓
//
// 任一步失败都不会回滚，但只要 abandoned_torrent 写入成功就返回 true 表示
// "已永久放弃"——这才是核心防御点。qBit 删除失败下次同步还能再删，
// download 行删除失败下次循环还能再删。
func (s *QBitSyncer) abandonDeadTorrent(ctx context.Context, dl *model.Download, reason string) bool {
	hash := ""
	if dl.InfoHash != nil {
		hash = strings.ToUpper(*dl.InfoHash)
	}
	if hash == "" {
		return false
	}

	// 1. 从 qBit 删除（带文件）
	if err := s.deleteFromQBit(ctx, hash); err != nil {
		zap.L().Warn("从 qBit 删除死种失败（继续其他步骤）",
			zap.String("hash", hash), zap.Error(err))
	}

	// 2. 写黑名单（先写黑名单再删 download 行：万一删 download 行后宕机，
	//    黑名单已经在，下一轮还能正确跳过同 hash 候选）
	row := model.AbandonedTorrent{
		InfoHash:    hash,
		AnimeID:     dl.AnimeID,
		Title:       dl.Name,
		Reason:      reason,
		Kind:        model.FailureKindTransient, // 死种判定都是"暂时找不到 peer"，留 TTL 给它复活机会
		AbandonedAt: time.Now(),
	}
	// ON CONFLICT DO NOTHING：同 hash 重复拉黑不报错
	if err := s.db.WithContext(ctx).
		Where("info_hash = ?", hash).
		FirstOrCreate(&row).Error; err != nil {
		zap.L().Warn("写黑名单失败", zap.String("hash", hash), zap.Error(err))
		return false
	}

	// 3. 删除 download 行
	if err := s.db.WithContext(ctx).
		Where("id = ?", dl.ID).
		Delete(&model.Download{}).Error; err != nil {
		zap.L().Warn("删除死种 download 行失败",
			zap.Uint("id", dl.ID), zap.Error(err))
		// 黑名单已写，允许下一轮再尝试删
	}

	zap.L().Info("永久放弃死种",
		zap.Uint("id", dl.ID),
		zap.Uint("anime_id", uintOrZero(dl.AnimeID)),
		zap.String("hash", hash),
		zap.String("name", dl.Name),
		zap.String("reason", reason))
	return true
}

func uintOrZero(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

// deleteFromQBit 调用 qBit /api/v2/torrents/delete 删除种子（含文件）
func (s *QBitSyncer) deleteFromQBit(ctx context.Context, hash string) error {
	data := url.Values{}
	data.Set("hashes", strings.ToLower(hash))
	data.Set("deleteFiles", "true")
	req, err := http.NewRequestWithContext(ctx, "POST",
		s.baseURL+"/api/v2/torrents/delete", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", s.baseURL)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("调用 qBit 删除接口: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qBit 删除返回 status=%d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// mapQBitState 把 qBit 状态映射到我们的 status
func mapQBitState(qs string) string {
	switch qs {
	case "uploading", "stalledUP", "queuedUP", "forcedUP", "checkingUP", "pausedUP":
		return model.DownloadStatusCompleted
	case "downloading", "metaDL", "stalledDL", "forcedDL", "checkingDL":
		return model.DownloadStatusDownloading
	case "pausedDL":
		return model.DownloadStatusPaused
	case "queuedDL":
		return model.DownloadStatusPending
	case "error", "missingFiles":
		return model.DownloadStatusFailed
	}
	return ""
}

func (s *QBitSyncer) ensureLogin(ctx context.Context) error {
	data := url.Values{}
	data.Set("username", s.user)
	data.Set("password", s.pass)
	req, err := http.NewRequestWithContext(ctx, "POST",
		s.baseURL+"/api/v2/auth/login", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", s.baseURL)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("qBit 登录失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || strings.Contains(string(body), "Fails") {
		return fmt.Errorf("qBit 登录失败 status=%d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *QBitSyncer) listTorrents(ctx context.Context) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		s.baseURL+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", s.baseURL)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 qBit 种子失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("查询 qBit 种子失败 status=%d: %s", resp.StatusCode, string(b))
	}
	var arr []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		return nil, err
	}
	return arr, nil
}
