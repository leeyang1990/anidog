package setting

import "testing"

func TestOnChangeNotifiesRegisteredCallback(t *testing.T) {
	service := &Service{callbacks: make(map[string][]func(string))}
	got := ""
	service.OnChange("http_proxy", func(value string) {
		got = value
	})

	service.notify("http_proxy", "http://127.0.0.1:8080")
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("callback value = %q", got)
	}
}
