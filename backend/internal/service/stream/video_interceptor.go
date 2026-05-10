package stream

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/config"
)

// InterceptedVideo 拦截到的视频信息
type InterceptedVideo struct {
	URL       string `json:"url"`
	VideoType string `json:"video_type"` // m3u8 / mp4
	Referer   string `json:"referer,omitempty"`
	Headers   string `json:"headers,omitempty"`
}

// 广告域名黑名单
var adDomains = []string{
	"googlesyndication.com",
	"doubleclick.net",
	"googleadservices.com",
	"google-analytics.com",
	"googletagmanager.com",
}

// VideoInterceptor rod 浏览器视频拦截器
type VideoInterceptor struct {
	cfg     *config.Config
	browser *rod.Browser
	mu      sync.Mutex
	// 信号量限制并发拦截数（rod page 太多会拖垮 chromium）
	sem chan struct{}
}

func NewVideoInterceptor(cfg *config.Config) *VideoInterceptor {
	maxConcurrent := cfg.StreamMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	return &VideoInterceptor{
		cfg: cfg,
		sem: make(chan struct{}, maxConcurrent),
	}
}

// Start 启动浏览器
func (v *VideoInterceptor) Start() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	l := launcher.New().Headless(v.cfg.RodHeadless)
	// 如果环境变量指定了 chromium 路径（例如 Docker 容器内），使用它
	if binPath := os.Getenv("ROD_BROWSER_BIN"); binPath != "" {
		l = l.Bin(binPath)
	}
	// 容器中无 /dev/shm，必须加此参数
	l = l.Set("no-sandbox").Set("disable-dev-shm-usage")

	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("rod 浏览器启动失败: %w", err)
	}

	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("rod 浏览器连接失败: %w", err)
	}

	v.browser = browser
	zap.L().Info("rod 浏览器已启动", zap.Bool("headless", v.cfg.RodHeadless))
	return nil
}

// Close 关闭浏览器
func (v *VideoInterceptor) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.browser != nil {
		v.browser.MustClose()
		v.browser = nil
		zap.L().Info("rod 浏览器已关闭")
	}
}

// GetBrowser 返回共享的 rod 浏览器实例，未启动时返回 nil。
func (v *VideoInterceptor) GetBrowser() *rod.Browser {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.browser
}

// InterceptVideoURL 拦截页面中的视频 URL，失败时自动重试最多 maxAttempts 次，每次 timeout 递增
func (v *VideoInterceptor) InterceptVideoURL(ctx context.Context, pageURL, referer, userAgent string) (*InterceptedVideo, error) {
	const maxAttempts = 3
	baseTimeout := time.Duration(v.cfg.StreamInterceptTimeout) * time.Second
	if baseTimeout <= 0 {
		baseTimeout = 30 * time.Second
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			zap.L().Info("视频拦截重试", zap.String("pageURL", pageURL), zap.Int("attempt", attempt))
			// 重试前短暂等待
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		// 每次递增超时：30s → 60s → 90s（给慢加载/广告跳转的页面更多时间）
		timeout := baseTimeout * time.Duration(attempt)
		video, err := v.interceptOnce(ctx, pageURL, referer, userAgent, timeout)
		if err == nil {
			return video, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// interceptOnce 单次尝试拦截（原有实现重命名）
func (v *VideoInterceptor) interceptOnce(ctx context.Context, pageURL, referer, userAgent string, timeout time.Duration) (*InterceptedVideo, error) {
	// 获取信号量（限制并发拦截数）
	select {
	case v.sem <- struct{}{}:
		defer func() { <-v.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	v.mu.Lock()
	browser := v.browser
	v.mu.Unlock()

	if browser == nil {
		return nil, fmt.Errorf("浏览器未启动")
	}

	zap.L().Info("开始拦截视频 URL", zap.String("pageURL", pageURL))

	// 创建新页面
	page := browser.MustPage("")
	defer page.MustClose()

	// 设置 User-Agent
	if userAgent != "" {
		page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: userAgent,
		})
	}

	// 拦截结果通道
	resultCh := make(chan *InterceptedVideo, 1)

	tryReport := func(v *InterceptedVideo) {
		select {
		case resultCh <- v:
		default:
		}
	}

	// 监听 Request 事件：URL 匹配视频路径
	go page.EachEvent(func(e *proto.NetworkRequestWillBeSent) bool {
		reqURL := e.Request.URL

		// 过滤广告/非视频
		for _, adDomain := range adDomains {
			if strings.Contains(reqURL, adDomain) {
				return false
			}
		}
		if isAsset(reqURL) {
			return false
		}

		if vtype := classifyByURL(reqURL); vtype != "" {
			tryReport(&InterceptedVideo{URL: reqURL, VideoType: vtype, Referer: referer})
			return true
		}
		return false
	})()

	// 监听 Response 事件：Content-Type 是 video/* 或 hls
	go page.EachEvent(func(e *proto.NetworkResponseReceived) bool {
		resp := e.Response
		reqURL := resp.URL
		ct := ""
		if resp.Headers != nil {
			if v, ok := resp.Headers["Content-Type"]; ok {
				ct = v.Str()
			} else if v, ok := resp.Headers["content-type"]; ok {
				ct = v.Str()
			}
		}
		if ct == "" {
			return false
		}
		if isAsset(reqURL) {
			return false
		}

		ctLower := strings.ToLower(ct)
		if strings.Contains(ctLower, "mpegurl") || strings.Contains(ctLower, "hls") {
			tryReport(&InterceptedVideo{URL: reqURL, VideoType: "m3u8", Referer: referer})
			return true
		}
		if strings.HasPrefix(ctLower, "video/") {
			tryReport(&InterceptedVideo{URL: reqURL, VideoType: "mp4", Referer: referer})
			return true
		}
		return false
	})()

	// 导航到页面
	if err := page.Navigate(pageURL); err != nil {
		return nil, fmt.Errorf("导航到页面失败: %w", err)
	}

	// 不等 WaitStable（有些站点永远不稳定），直接等待拦截结果
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	select {
	case video := <-resultCh:
		zap.L().Info("视频 URL 拦截成功",
			zap.String("type", video.VideoType),
			zap.String("page", pageURL),
			zap.String("url", video.URL))
		return video, nil
	case <-time.After(timeout):
		zap.L().Warn("视频拦截超时", zap.String("page", pageURL), zap.Duration("timeout", timeout))
		return nil, fmt.Errorf("视频拦截超时 (%v)", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// classifyByURL 通过 URL 模式判断是否为视频流
func classifyByURL(u string) string {
	if strings.Contains(u, ".m3u8") {
		return "m3u8"
	}
	if strings.Contains(u, ".mp4") && !strings.Contains(u, ".mp4/") {
		return "mp4"
	}
	// 常见视频 CDN 路径模式
	lower := strings.ToLower(u)
	for _, pat := range []string{"/hls/", "/vod/", "playlist.m3u8"} {
		if strings.Contains(lower, pat) {
			return "m3u8"
		}
	}
	return ""
}

// isAsset 是否为静态资源 / 非视频请求
func isAsset(u string) bool {
	lower := strings.ToLower(u)
	// 明显的静态资源
	for _, ext := range []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".otf", ".json"} {
		if strings.HasSuffix(lower, ext) || strings.Contains(lower, ext+"?") {
			return true
		}
	}
	return false
}
