package stream

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/model"
)

// InterceptedVideo 拦截到的视频信息
type InterceptedVideo struct {
	ProxyURL  string `json:"-"`
	URL       string `json:"url"`
	VideoType string `json:"video_type"` // m3u8 / mp4
	Referer   string `json:"referer,omitempty"`
	Headers   string `json:"headers,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
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
	sem      chan struct{}
	proxyURL func() string
	contexts map[string]*rod.Browser
}

func NewVideoInterceptor(cfg *config.Config, proxies ...func() string) *VideoInterceptor {
	proxy := func() string { return cfg.HTTPProxy }
	if len(proxies) > 0 && proxies[0] != nil {
		proxy = proxies[0]
	}
	maxConcurrent := cfg.StreamMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	return &VideoInterceptor{
		cfg:      cfg,
		sem:      make(chan struct{}, maxConcurrent),
		proxyURL: proxy, contexts: make(map[string]*rod.Browser),
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
		_ = v.browser.Close()
		v.browser = nil
		v.contexts = make(map[string]*rod.Browser)
		zap.L().Info("rod 浏览器已关闭")
	}
}

// GetBrowser 返回共享的 rod 浏览器实例，未启动时返回 nil。
func (v *VideoInterceptor) GetBrowser() *rod.Browser {
	return v.browserForProxy(v.proxyURL())
}
func (v *VideoInterceptor) GetBrowserForRule(rule *model.StreamRule) *rod.Browser {
	proxy := v.proxyURL()
	if rule != nil && rule.ProxyMode == "direct" {
		proxy = ""
	}
	return v.browserForProxy(proxy)
}
func (v *VideoInterceptor) browserForProxy(proxy string) *rod.Browser {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.browser == nil {
		return nil
	}
	if browser := v.contexts[proxy]; browser != nil {
		return browser
	}
	proxyServer := proxy
	if proxy != "" {
		parsed, err := url.Parse(proxy)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil
		}
		// Chromium's context proxy does not accept embedded credentials. Keep them
		// out of process arguments and handle its proxy authentication challenge.
		if parsed.User != nil {
			username := parsed.User.Username()
			password, _ := parsed.User.Password()
			go v.browser.HandleAuth(username, password)()
			parsed.User = nil
		}
		if parsed.Scheme == "socks5h" {
			parsed.Scheme = "socks5"
		}
		proxyServer = parsed.String()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := (proto.TargetCreateBrowserContext{ProxyServer: proxyServer}).Call(v.browser.Context(ctx))
	if err != nil {
		zap.L().Warn("创建流媒体浏览器网络上下文失败", zap.Error(err))
		return nil
	}
	browser := *v.browser
	browser.BrowserContextID = res.BrowserContextID
	v.contexts[proxy] = &browser
	return &browser
}

type playerFailure struct{ message string }

func (e *playerFailure) Error() string { return "播放器已报错：" + e.message }

// InterceptVideoURL 拦截页面中的视频 URL，失败时自动重试最多 maxAttempts 次，每次 timeout 递增
func (v *VideoInterceptor) InterceptVideoURL(ctx context.Context, pageURL, referer, userAgent string, proxyModes ...string) (*InterceptedVideo, error) {
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
		proxy := v.proxyURL()
		if len(proxyModes) > 0 && proxyModes[0] == "direct" {
			proxy = ""
		}
		video, err := v.interceptOnce(ctx, pageURL, referer, userAgent, timeout, proxy)
		if err == nil {
			return video, nil
		}
		lastErr = err
		var failed *playerFailure
		if errors.As(err, &failed) {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// interceptOnce 单次尝试拦截（原有实现重命名）
func (v *VideoInterceptor) interceptOnce(ctx context.Context, pageURL, referer, userAgent string, timeout time.Duration, proxyURLs ...string) (*InterceptedVideo, error) {
	select {
	case v.sem <- struct{}{}:
		defer func() { <-v.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	proxy := v.proxyURL()
	if len(proxyURLs) > 0 {
		proxy = proxyURLs[0]
	}
	browser := v.browserForProxy(proxy)
	if browser == nil {
		return nil, fmt.Errorf("流媒体浏览器或代理上下文不可用")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	page, err := browser.Context(ctx).Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, err
	}
	defer func() { _ = page.Context(context.Background()).Close() }()
	if userAgent != "" {
		if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: userAgent}); err != nil {
			return nil, err
		}
	}
	resultCh := make(chan *InterceptedVideo, 1)
	failureCh := make(chan error, 1)
	// A successful media response is required; requests alone also include broken
	// manifests, advertising placeholders and 403 responses.
	go page.EachEvent(func(e *proto.NetworkResponseReceived) bool {
		r := e.Response
		if isAsset(r.URL) {
			return false
		}
		for _, domain := range adDomains {
			if strings.Contains(r.URL, domain) {
				return false
			}
		}
		ct := strings.ToLower(r.Headers["content-type"].Str() + r.Headers["Content-Type"].Str())
		kind := mediaResponseType(r.URL, ct)
		if kind == "" {
			return false
		}
		if r.Status >= 400 {
			select {
			case failureCh <- &playerFailure{fmt.Sprintf("媒体请求返回 HTTP %d", r.Status)}:
			default:
			}
			return true
		}
		if r.Status < 200 || r.Status >= 300 {
			return false
		}
		ref := referer
		agent := userAgent
		for name, value := range r.RequestHeaders {
			if strings.EqualFold(name, "Referer") && value.Str() != "" {
				ref = value.Str()
			}
			if strings.EqualFold(name, "User-Agent") && value.Str() != "" {
				agent = value.Str()
			}
		}
		if ref == "" {
			ref = pageURL
		}
		video := &InterceptedVideo{URL: r.URL, VideoType: kind, Referer: ref, UserAgent: agent, ProxyURL: proxy}
		select {
		case resultCh <- video:
		default:
		}
		return true
	})()
	if err := page.Navigate(pageURL); err != nil {
		return nil, fmt.Errorf("导航到播放页失败: %w", err)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case video := <-resultCh:
			zap.L().Info("视频 URL 拦截成功", zap.String("page", pageURL), zap.String("type", video.VideoType))
			return video, nil
		case err := <-failureCh:
			return nil, err
		case <-ticker.C:
			if reason := inspectPlayerFailure(page, 0); reason != "" {
				return nil, &playerFailure{reason}
			}
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("视频拦截超时 (%v)", timeout)
			}
			return nil, ctx.Err()
		}
	}
}

func mediaResponseType(rawURL, contentType string) string {
	lower := strings.ToLower(strings.SplitN(rawURL, "?", 2)[0])
	if strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".m4s") {
		return ""
	}
	if strings.Contains(contentType, "mpegurl") || strings.HasSuffix(lower, ".m3u8") {
		return "m3u8"
	}
	if strings.HasSuffix(lower, ".mp4") || strings.Contains(contentType, "video/mp4") || strings.Contains(contentType, "video/webm") {
		return "mp4"
	}
	return ""
}

// Inspect only short, visible error messages, including nested player frames.
// Long outer pages often mention errors in help text and must not be rejected.
func playerFailureText(text string) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) > 500 {
		return ""
	}
	for _, marker := range []string{"播放失败，请刷新或稍候再试", "播放失败，请刷新", "未取得有效播放地址", "解析接口返回无效", "解析请求异常", "播放器组件加载失败", "The media could not be loaded"} {
		if strings.Contains(text, marker) {
			return marker
		}
	}
	return ""
}

func inspectPlayerFailure(page *rod.Page, depth int) string {
	if depth > 4 {
		return ""
	}
	result, err := page.Eval(`() => document.body ? document.body.innerText : ""`)
	if err == nil {
		if text := playerFailureText(result.Value.Str()); text != "" {
			return text
		}
	}
	frames, err := page.Elements("iframe")
	if err != nil {
		return ""
	}
	for _, el := range frames {
		frame, err := el.Frame()
		if err == nil {
			if reason := inspectPlayerFailure(frame, depth+1); reason != "" {
				return reason
			}
		}
	}
	return ""
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
