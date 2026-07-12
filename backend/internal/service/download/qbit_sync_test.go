package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQBitSyncLoginAcceptsNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/auth/login" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	s := &QBitSyncer{baseURL: server.URL, user: "admin", pass: "secret", client: server.Client()}
	if err := s.ensureLogin(context.Background()); err != nil {
		t.Fatalf("204 login should succeed: %v", err)
	}
}
