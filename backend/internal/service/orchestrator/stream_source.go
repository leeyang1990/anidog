package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/stream"
)

// streamCatalog isolates source discovery from execution and makes fallback
// behavior testable without launching a browser or a real download.
type streamCatalog interface {
	SearchAnime(context.Context, *model.StreamRule, string) ([]stream.SearchResult, error)
	GetEpisodes(context.Context, *model.StreamRule, string) ([]stream.EpisodeInfo, error)
}

type streamChoice struct {
	rule      *model.StreamRule
	detailURL string
	episode   stream.EpisodeInfo
	count     int
	title     string
}

func (o *Orchestrator) tryStream(ctx context.Context, anime *model.Anime, ep, retryCount int) (bool, int, string) {
	if o.streamMgr == nil {
		return false, 0, "stream manager 未就绪"
	}
	if o.isDuplicate(ctx, anime.ID, ep, SourceStream) {
		return false, 0, "该集已有 stream 记录或在失败冷却期内，跳过"
	}
	choice, err := o.resolveStreamSource(ctx, anime, ep)
	if err != nil {
		return false, 0, err.Error()
	}
	task := &dlservice.Task{
		Name: fmt.Sprintf("%s - 第%02d集", anime.Title, ep),
		URL:  choice.episode.URL, SourceTitle: choice.title, DownloadType: model.DownloadTypeStream,
		Source: SourceStream, AnimeName: anime.Title, AnimeID: &anime.ID,
		EpisodeNumber: &ep, StreamRuleID: &choice.rule.ID, StreamRule: choice.rule,
		StreamDetailURL: choice.detailURL, StreamRoadName: choice.episode.RoadName,
		RetryCount: retryCount,
	}
	if _, err := o.dlSvc.Create(ctx, task); err != nil {
		return false, choice.count, "创建下载任务失败: " + err.Error()
	}
	// Remember only a source that was actually queued. A failed search or DB
	// error must not mutate the anime's preferred source.
	updates := map[string]interface{}{
		"stream_rule_id": choice.rule.ID, "stream_rule_name": choice.rule.Name,
		"stream_detail_url": choice.detailURL, "stream_road_name": choice.episode.RoadName,
		"source_health_status": "", "source_health_note": "", "source_health_at": nil,
	}
	if err := o.db.WithContext(ctx).Model(anime).Updates(updates).Error; err != nil {
		// The task already exists; reporting enqueue failure would invite duplicates.
		return true, choice.count, "已入队，但保存首选流媒体源失败: " + err.Error()
	}
	return true, choice.count, fmt.Sprintf("已从线路 %s 入队: %s", choice.episode.RoadName, choice.episode.Name)
}

// resolveStreamSource applies exactly the same candidate policy to the current
// and backup rules. Failure of one road does not blacklist the whole website.
func (o *Orchestrator) resolveStreamSource(ctx context.Context, anime *model.Anime, ep int) (*streamChoice, error) {
	var rules []model.StreamRule
	if err := o.db.WithContext(ctx).Where("enabled = ?", true).Order("id ASC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("查询 stream 规则失败: %w", err)
	}
	sort.SliceStable(rules, func(i, j int) bool {
		currentI := anime.StreamRuleID != nil && rules[i].ID == *anime.StreamRuleID
		currentJ := anime.StreamRuleID != nil && rules[j].ID == *anime.StreamRuleID
		if currentI != currentJ {
			return currentI
		}
		return streamRuleHealthScore(&rules[i]) > streamRuleHealthScore(&rules[j])
	})
	for i := range rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rule := &rules[i]
		if isRuleBroken(rule) {
			continue
		}
		detail, preferred := "", ""
		if anime.StreamRuleID != nil && rule.ID == *anime.StreamRuleID {
			if anime.StreamDetailURL != nil {
				detail = *anime.StreamDetailURL
			}
			if anime.StreamRoadName != nil {
				preferred = *anime.StreamRoadName
			}
		}
		// Persisted URLs are hints, not verified identity. Recheck the catalog
		// before reuse so an old wrong-arc binding cannot perpetuate itself.
		searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		results, err := o.streamMgr.SearchAnime(searchCtx, rule, anime.Title)
		cancel()
		if err != nil {
			continue
		}
		best := stream.PickBestMatch(anime.Title, results)
		if best == nil {
			continue
		}
		for j := range results {
			if results[j].URL == detail && stream.IsConfidentMatch(anime.Title, results[j].Name) {
				best = &results[j]
				break
			}
		}
		if detail != best.URL {
			preferred = ""
		}
		detail = best.URL
		parseCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		episodes, err := o.streamMgr.GetEpisodes(parseCtx, rule, detail)
		cancel()
		if err != nil {
			continue
		}
		blocked, err := o.coolingStreamRoads(ctx, anime.ID, ep, rule.ID, detail, time.Now())
		if err != nil {
			return nil, err
		} // DB failure must not bypass cooldown.
		item, _, ok := selectStreamEpisode(episodes, ep, preferred, blocked, rule.MultiSources)
		if ok {
			return &streamChoice{rule: rule, detailURL: detail, episode: item, count: len(episodes), title: best.Name}, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("所有启用的流媒体源均无可用的第 %d 集候选（缺集、解析失败或冷却中），等待下次搜索", ep)
}

// coolingStreamRoads uses stable rule/detail/road identity, not expiring CDN
// URLs. Legacy rows lacking detail/road fall back to their narrower available
// identity; they expire as well, rather than becoming permanent rule bans.
func (o *Orchestrator) coolingStreamRoads(ctx context.Context, animeID uint, ep int, ruleID uint, detail string, now time.Time) (map[string]bool, error) {
	var failures []model.Download
	err := o.db.WithContext(ctx).
		Where("anime_id = ? AND episode_number = ? AND stream_rule_id = ? AND download_type = ? AND status = ?",
			animeID, ep, ruleID, model.DownloadTypeStream, model.DownloadStatusFailed).
		Where("stream_detail_url = ? OR stream_detail_url IS NULL OR stream_detail_url = ''", detail).
		Order("id DESC").Find(&failures).Error
	if err != nil {
		return nil, fmt.Errorf("查询线路冷却失败: %w", err)
	}
	blocked, seen := map[string]bool{}, map[string]bool{}
	for _, failure := range failures {
		road := ""
		if failure.StreamRoadName != nil {
			road = *failure.StreamRoadName
		}
		if seen[road] {
			continue
		}
		seen[road] = true
		if now.Before(streamCandidateRetryAt(failure)) {
			if road == "" {
				blocked["*"] = true
			} else {
				blocked[road] = true
			}
		}
	}
	return blocked, nil
}

// Candidate cooldown is independent of recovery-search polling. A rejected
// video can be replaced by the publisher; allow a daily probe, not an endless
// immediate loop. Permanent local errors wait for repair and a six-hour probe.
func streamCandidateRetryAt(failure model.Download) time.Time {
	delay := 6 * time.Hour
	switch failure.FailureKind {
	case model.FailureKindTransient:
		delay = dlservice.RetryDelay(failure.RetryCount)
		if delay <= 0 {
			delay = time.Hour
		}
	case model.FailureKindExhausted:
		delay = time.Hour
	case model.FailureKindRejected:
		delay = 24 * time.Hour
	}
	return failure.UpdatedAt.Add(delay)
}
