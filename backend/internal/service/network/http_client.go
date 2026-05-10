package network

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/anidog/anidog-go/internal/config"
)

// HTTPClient 共享 HTTP 客户端
type HTTPClient struct {
	client *http.Client
	cfg    *config.Config
}

func NewHTTPClient(cfg *config.Config) *HTTPClient {
	transport := &http.Transport{}
	if cfg.HTTPProxy != "" {
		if proxyURL, err := url.Parse(cfg.HTTPProxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &HTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		cfg: cfg,
	}
}

func (c *HTTPClient) Client() *http.Client {
	return c.client
}

func (c *HTTPClient) Get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AniDog/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *HTTPClient) Post(ctx context.Context, url, contentType string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "AniDog/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *HTTPClient) GetText(ctx context.Context, url string) (string, error) {
	data, err := c.Get(ctx, url)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
