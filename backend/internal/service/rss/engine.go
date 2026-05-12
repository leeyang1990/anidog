package rss

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/model"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/titleparse"
)

// Engine fetches RSS feeds, matches entries against rules, and triggers downloads.
type Engine struct {
	db        *gorm.DB
	dlSvc     dlservice.Downloader
	parser    *Parser
	matcher   *Matcher
	mediaRoot string // 下载根目录，用于按番剧组织子目录
}

// NewEngine creates a new RSS engine.
func NewEngine(db *gorm.DB, dlSvc dlservice.Downloader) *Engine {
	return &Engine{
		db:      db,
		dlSvc:   dlSvc,
		parser:  NewParser(),
		matcher: NewMatcher(),
	}
}

// SetMediaRoot 设置下载根目录，用于生成番剧子目录路径
func (e *Engine) SetMediaRoot(root string) {
	e.mediaRoot = root
}

// CheckFeed fetches and processes a single RSS feed.
func (e *Engine) CheckFeed(ctx context.Context, feed *model.RSSFeed) (int, error) {
	if !feed.Enabled {
		return 0, nil
	}

	items, err := e.parser.Parse(ctx, feed.URL)
	if err != nil {
		return 0, err
	}

	// Load rules for this feed
	var rules []model.RSSRule
	e.db.Where("rss_feed_id = ?", feed.ID).Find(&rules)

	newCount := 0
	for _, item := range items {
		// Dedup: check if we've already seen this entry
		var existing model.RSSEntry
		if e.db.Where("entry_id = ? AND rss_feed_id = ?", item.EntryID, feed.ID).First(&existing).Error == nil {
			continue
		}

		// 解析标题提取字幕组/番名/集数
		parsed := titleparse.Parse(item.Title)

		// Record the entry（含解析结果）
		entry := model.RSSEntry{
			EntryID:       item.EntryID,
			Title:         item.Title,
			Link:          item.Link,
			Published:     item.Published,
			RSSFeedID:     feed.ID,
			ParsedEpisode: parsed.EpisodeNum,
		}
		if parsed.AnimeName != "" {
			entry.ParsedAnime = &parsed.AnimeName
		}
		if parsed.Group != "" {
			entry.ParsedGroup = &parsed.Group
		}

		// 关联已订阅番剧（常规路径）
		var matchedAnime *model.Anime
		if parsed.AnimeName != "" {
			matchedAnime = e.findSubscribedAnime(parsed.AnimeName)
		}
		if matchedAnime == nil {
			matchedAnime = e.findByTitleSubstring(item.Title)
		}

		if matchedAnime != nil {
			entry.MatchedAnimeID = &matchedAnime.ID
		}
		e.db.Create(&entry)

		// Match against rules
		if !e.matcher.Match(item.Title, rules) {
			continue
		}

		// Create download task via the unified service
		task := &dlservice.Task{
			Name:         item.Title,
			URL:          item.Link,
			DownloadType: model.DownloadTypeTorrent,
			Source:       dlservice.SourceRSS,
		}
		if matchedAnime != nil {
			task.AnimeID = &matchedAnime.ID
			task.AnimeName = matchedAnime.Title
			// 按番剧名组织子目录
			task.SavePath = dlservice.BuildAnimeSavePath(e.mediaRoot, matchedAnime)
		}
		if parsed.EpisodeNum != nil {
			task.EpisodeNumber = parsed.EpisodeNum
		}

		if _, err := e.dlSvc.Create(ctx, task); err != nil {
			zap.L().Error("RSS 创建下载任务失败", zap.String("title", item.Title), zap.Error(err))
			continue
		}

		// Mark entry as downloaded
		e.db.Model(&entry).Update("downloaded", true)
		newCount++
	}

	// Update last check time
	now := time.Now()
	e.db.Model(feed).Update("last_check", &now)

	zap.L().Info("RSS 检查完成", zap.String("feed", feed.Name), zap.Int("new", newCount), zap.Int("total", len(items)))
	return newCount, nil
}

// RefreshAll checks all enabled RSS feeds.
func (e *Engine) RefreshAll(ctx context.Context) {
	var feeds []model.RSSFeed
	e.db.Where("enabled = ?", true).Find(&feeds)

	zap.L().Info("开始刷新所有 RSS 订阅源", zap.Int("count", len(feeds)))

	for i := range feeds {
		if _, err := e.CheckFeed(ctx, &feeds[i]); err != nil {
			zap.L().Error("RSS 检查失败", zap.String("feed", feeds[i].Name), zap.Error(err))
		}
	}
}

// ParseFeedPreview parses a feed URL without creating downloads (for preview/test).
func (e *Engine) ParseFeedPreview(ctx context.Context, feedURL string) ([]ParsedItem, error) {
	return e.parser.Parse(ctx, feedURL)
}

// findSubscribedAnime 按解析的番名在订阅番剧里精确/模糊匹配
func (e *Engine) findSubscribedAnime(name string) *model.Anime {
	if name == "" {
		return nil
	}
	var animes []model.Anime
	e.db.Where("is_subscribed = ?", true).Find(&animes)
	for i := range animes {
		a := &animes[i]
		if a.Title == name {
			return a
		}
		if containsMatch(name, a.Title) || containsMatch(a.Title, name) {
			return a
		}
	}
	return nil
}

// findByTitleSubstring 原有的松散匹配（兼容老逻辑）
func (e *Engine) findByTitleSubstring(title string) *model.Anime {
	var animes []model.Anime
	e.db.Where("is_subscribed = ?", true).Find(&animes)
	for i := range animes {
		a := &animes[i]
		if a.Title != "" && containsMatch(title, a.Title) {
			return a
		}
	}
	return nil
}
// containsMatch does a case-insensitive substring check.
func containsMatch(title, keyword string) bool {
	return len(keyword) > 0 && searchStr(title, keyword)
}

func searchStr(s, substr string) bool {
	ls := toLower(s)
	lsub := toLower(substr)
	return len(lsub) <= len(ls) && containsLower(ls, lsub)
}

func toLower(s string) string {
	return stringsToLower(s)
}

func stringsToLower(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result = append(result, c)
	}
	return string(result)
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
