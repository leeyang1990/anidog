package qbittorrent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginAcceptsNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/auth/login" {
			http.NotFound(w, r)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	q := &QBittorrent{client: server.Client(), config: &Config{Username: "admin", Password: "secret"}, baseURL: server.URL}
	if err := q.login(); err != nil {
		t.Fatalf("204 login should succeed: %v", err)
	}
	if q.sessionID != "session" {
		t.Fatalf("expected SID to be captured, got %q", q.sessionID)
	}
}

func TestLoginRejectsFailsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Fails."))
	}))
	defer server.Close()

	q := &QBittorrent{client: server.Client(), config: &Config{}, baseURL: server.URL}
	if err := q.login(); err == nil {
		t.Fatal("Fails response must be rejected")
	}
}
