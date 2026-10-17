package mediacheck

import (
	"context"
	"strings"
	"testing"
)

func TestMediaRouteSurvivesEnvironmentConflicts(t *testing.T) {
	t.Setenv("http_proxy", "http://wrong.invalid:8080")
	t.Setenv("HTTPS_PROXY", "http://wrong.invalid:8080")
	t.Setenv("ALL_PROXY", "socks5://wrong.invalid:1080")
	t.Setenv("NO_PROXY", "*")
	for _, route := range []string{"", "http://user:private@right.invalid:8080"} {
		cmd := MediaCommand(WithHTTPProxy(context.Background(), route), "ffprobe", "input")
		if strings.Contains(strings.Join(cmd.Args, " "), "private") {
			t.Fatal("proxy credentials escaped into command arguments")
		}
		env := map[string]string{}
		for _, entry := range cmd.Env {
			k, v, _ := strings.Cut(entry, "=")
			env[k] = v
			if strings.Contains(v, "wrong.invalid") {
				t.Fatal("inherited proxy conflicts with the media route")
			}
		}
		for _, key := range []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"} {
			if env[key] != route {
				t.Fatalf("%s=%q, want %q", key, env[key], route)
			}
		}
		if env["NO_PROXY"] != "" || env["ALL_PROXY"] != "" {
			t.Fatal("inherited bypass or SOCKS route remained")
		}
	}
}
