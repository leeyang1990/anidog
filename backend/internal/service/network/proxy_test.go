package network

import (
	"net/http"
	"testing"
)

func TestProxyProviderUpdatesWithoutRebuildingClient(t *testing.T) {
	provider := NewProxyProvider("")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)

	proxyURL, err := provider.Proxy(req)
	if err != nil || proxyURL != nil {
		t.Fatalf("initial proxy = %v, %v; want nil, nil", proxyURL, err)
	}

	provider.Set("http://127.0.0.1:8080")
	proxyURL, err = provider.Proxy(req)
	if err != nil {
		t.Fatalf("updated proxy error: %v", err)
	}
	if got := proxyURL.String(); got != "http://127.0.0.1:8080" {
		t.Fatalf("updated proxy = %q", got)
	}

	provider.Set("")
	proxyURL, err = provider.Proxy(req)
	if err != nil || proxyURL != nil {
		t.Fatalf("cleared proxy = %v, %v; want nil, nil", proxyURL, err)
	}
}
