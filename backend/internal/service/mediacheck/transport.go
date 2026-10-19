package mediacheck

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type mediaProxyKey struct{}

// WithHTTPProxy freezes the route used to obtain this media URL. An explicit
// empty value means direct access, including when the process has proxy envs.
func WithHTTPProxy(ctx context.Context, proxy string) context.Context {
	return context.WithValue(ctx, mediaProxyKey{}, proxy)
}

// MediaCommand keeps browser, ffprobe, OCR frames and full transfers on the
// same route. Proxy credentials stay out of command arguments and diagnostics.
func MediaCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	proxy, configured := ctx.Value(mediaProxyKey{}).(string)
	if !configured {
		return cmd
	}
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil || u.Scheme != "http" || u.Hostname() == "" {
			// FFmpeg ignores SOCKS in http_proxy and silently connects directly.
			// SOCKS routes must first go through PrepareTransport's local bridge.
			cmd.Err = toolError{fmt.Errorf("媒体代理路由不可用：需要 HTTP 代理或已准备的 SOCKS 转接")}
			return cmd
		}
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToLower(key) {
		case "http_proxy", "https_proxy", "all_proxy", "no_proxy":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	for _, key := range []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"} {
		cmd.Env = append(cmd.Env, key+"="+proxy)
	}
	cmd.Env = append(cmd.Env, "ALL_PROXY=", "all_proxy=", "NO_PROXY=", "no_proxy=")
	return cmd
}
