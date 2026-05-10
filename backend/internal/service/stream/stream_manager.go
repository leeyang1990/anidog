package stream

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/network"
)

// StreamManager 流媒体下载管理器
type StreamManager struct {
	cfg         *config.Config
	db          *gorm.DB
	executor    *StreamRuleExecutor
	interceptor *VideoInterceptor
	downloader  *M3U8Downloader
	httpClient  *network.HTTPClient
}

var (
	streamManagerInstance *StreamManager
	streamManagerOnce     interface{ Do(func()) }
)

func NewStreamManager(cfg *config.Config, httpClient *network.HTTPClient, db *gorm.DB) *StreamManager {
	interceptor := NewVideoInterceptor(cfg)
	return &StreamManager{
		cfg:         cfg,
		db:          db,
		executor:    NewStreamRuleExecutor(httpClient, interceptor),
		interceptor: interceptor,
		downloader:  NewM3U8Downloader(cfg),
		httpClient:  httpClient,
	}
}

// Start 启动流媒体管理器（rod 浏览器）
func (m *StreamManager) Start() error {
	if err := m.interceptor.Start(); err != nil {
		zap.L().Warn("流媒体管理器启动失败 (流媒体功能不可用)", zap.Error(err))
		return err
	}
	zap.L().Info("流媒体管理器已启动")
	return nil
}

// Close 关闭流媒体管理器
func (m *StreamManager) Close() {
	m.interceptor.Close()
	zap.L().Info("流媒体管理器已关闭")
}

// SearchAnime 搜索番剧
func (m *StreamManager) SearchAnime(ctx context.Context, rule *model.StreamRule, keyword string) ([]SearchResult, error) {
	return m.executor.Search(ctx, rule, keyword)
}

// GetEpisodes 获取集数列表
func (m *StreamManager) GetEpisodes(ctx context.Context, rule *model.StreamRule, detailURL string) ([]EpisodeInfo, error) {
	return m.executor.ParseEpisodes(ctx, rule, detailURL)
}

// DownloadEpisode 下载单集。
//   - anime: 可选，用于按 Plex/Emby 规范生成路径。nil 时回退到 {base}/{animeName}/{ep}.mp4
//   - episodeNumber: 集数（1-based），从 Plex 规范生成文件名时必需
func (m *StreamManager) DownloadEpisode(ctx context.Context, episode *EpisodeInfo, rule *model.StreamRule, savePath, animeName string, anime *model.Anime, episodeNumber int, progressCB func(float64, int64)) (string, error) {
	// 1. 拦截视频 URL
	referer := ""
	if rule.Referer != nil {
		referer = *rule.Referer
	}
	userAgent := ""
	if rule.UserAgent != nil {
		userAgent = *rule.UserAgent
	}

	video, err := m.interceptor.InterceptVideoURL(ctx, episode.URL, referer, userAgent)
	if err != nil {
		return "", fmt.Errorf("拦截视频 URL 失败: %w", err)
	}

	// 2. 构建输出文件路径
	baseDir := savePath
	if baseDir == "" {
		baseDir = m.resolveBaseDir()
	}

	ext := ".mp4" // ffmpeg 输出统一 mp4

	var outputPath string
	if anime != nil && episodeNumber > 0 {
		// Plex/Emby 规范: {base}/{title} ({year})/Season {ss}/{title} S{ss}E{ee}.mp4
		outputPath = BuildMediaPath(baseDir, anime, episodeNumber, ext)
	} else {
		// 回退: {base}/{animeName}/{episodeName}.mp4
		outputPath = buildLegacyPath(baseDir, animeName, episode.Name, ext)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 3. 使用 ffmpeg 下载
	resultPath, err := m.downloader.Download(ctx, "", video.URL, outputPath, video.VideoType, video.Referer, progressCB)
	if err != nil {
		return "", fmt.Errorf("ffmpeg 下载失败: %w", err)
	}

	zap.L().Info("流媒体下载完成", zap.String("episode", episode.Name), zap.String("path", resultPath))
	return resultPath, nil
}

// CancelDownload 取消下载
func (m *StreamManager) CancelDownload(taskID string) bool {
	return m.downloader.Cancel(taskID)
}

// resolveBaseDir 从 DB settings 读取下载目录，fallback 到 config
func (m *StreamManager) resolveBaseDir() string {
	if m.db != nil {
		var s model.Setting
		if err := m.db.Where("key = ?", "download_dir").First(&s).Error; err == nil && s.Value != "" {
			return s.Value
		}
	}
	if m.cfg.StreamDownloadDir != "" {
		return m.cfg.StreamDownloadDir
	}
	if m.cfg.MediaRoot != "" {
		return m.cfg.MediaRoot
	}
	return "/downloads"
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}
