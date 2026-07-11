package network

import (
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// ProxyProvider provides a proxy function whose target can be changed without
// rebuilding http.Client or restarting the process.
type ProxyProvider struct {
	value atomic.Value // always stores a string
}

func NewProxyProvider(initial string) *ProxyProvider {
	p := &ProxyProvider{}
	p.value.Store(strings.TrimSpace(initial))
	return p
}

func (p *ProxyProvider) Set(rawURL string) {
	p.value.Store(strings.TrimSpace(rawURL))
}

func (p *ProxyProvider) Get() string {
	value, _ := p.value.Load().(string)
	return value
}

func (p *ProxyProvider) Proxy(_ *http.Request) (*url.URL, error) {
	rawURL := p.Get()
	if rawURL == "" {
		return nil, nil
	}
	return url.Parse(rawURL)
}

func NewClient(proxy *ProxyProvider, timeout time.Duration) *http.Client {
	transport := &http.Transport{}
	if proxy != nil {
		transport.Proxy = proxy.Proxy
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}
