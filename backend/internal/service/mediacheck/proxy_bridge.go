package mediacheck

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// PrepareTransport keeps FFmpeg on the browser's selected route. FFmpeg only
// supports HTTP proxies; a task-scoped loopback bridge handles SOCKS, including
// remote DNS and authentication. Its lifetime covers preflight, OCR and download.
func PrepareTransport(ctx context.Context, raw string) (context.Context, func(), error) {
	noop := func() {}
	if raw == "" {
		return WithHTTPProxy(ctx, ""), noop, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return ctx, noop, fmt.Errorf("媒体代理地址无效")
	}
	if u.Scheme == "http" {
		return WithHTTPProxy(ctx, raw), noop, nil
	}
	if u.Scheme != "socks5" && u.Scheme != "socks5h" {
		return ctx, noop, fmt.Errorf("媒体代理协议不支持：请选择 HTTP 或 SOCKS5")
	}
	u.Scheme = "socks5"
	dialer, err := proxy.FromURL(u, &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second})
	if err != nil {
		return ctx, noop, fmt.Errorf("无法准备 SOCKS 媒体代理")
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return ctx, noop, fmt.Errorf("SOCKS 媒体代理不支持取消")
	}
	bridgeCtx, cancel := context.WithCancel(ctx)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return ctx, noop, fmt.Errorf("无法启动媒体代理转接: %w", err)
	}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		ctx, stop := context.WithTimeout(ctx, 20*time.Second)
		defer stop()
		return contextDialer.DialContext(ctx, network, address)
	}
	transport := &http.Transport{DialContext: dial, ResponseHeaderTimeout: 20 * time.Second}
	forward := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.Header.Del("Proxy-Authorization")
			r.Out.Header.Del("Proxy-Connection")
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "media proxy upstream unavailable", http.StatusBadGateway)
		},
	}
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
		BaseContext: func(net.Listener) context.Context { return bridgeCtx },
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodConnect {
				forward.ServeHTTP(w, r)
				return
			}
			upstream, err := dial(r.Context(), "tcp", r.Host)
			if err != nil {
				http.Error(w, "media proxy tunnel unavailable", http.StatusBadGateway)
				return
			}
			defer upstream.Close()
			client, buffered, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer client.Close()
			stop := context.AfterFunc(bridgeCtx, func() { client.Close(); upstream.Close() })
			defer stop()
			if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
				return
			}
			if err := buffered.Flush(); err != nil {
				return
			}
			done := make(chan struct{})
			go func() { io.Copy(upstream, buffered); upstream.Close(); close(done) }()
			io.Copy(client, upstream)
			client.Close()
			<-done
		}),
	}
	var once sync.Once
	closeBridge := func() { once.Do(func() { cancel(); ln.Close(); srv.Close(); transport.CloseIdleConnections() }) }
	stop := context.AfterFunc(bridgeCtx, closeBridge)
	go func() { _ = srv.Serve(ln) }()
	return WithHTTPProxy(ctx, "http://"+ln.Addr().String()), func() { stop(); closeBridge() }, nil
}
