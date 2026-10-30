package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNyaaLanguageSelectionBelongsToSharedPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		if r.URL.Query().Get("c") != "1_0" {
			w.Write([]byte(`<rss><channel/></rss>`))
			return
		}
		w.Write([]byte(`<rss xmlns:nyaa="http://nyaa.si/ns"><channel><item><title>[Fixture] 示例番剧 - 05 [1080p][简中]</title><link>https://nyaa.si/view/42</link><nyaa:infoHash>0123456789012345678901234567890123456789</nyaa:infoHash><nyaa:seeders>12</nyaa:seeders><nyaa:size>400 MiB</nyaa:size></item></channel></rss>`))
	}))
	defer server.Close()
	got, err := (&NyaaIndexer{BaseURL: server.URL, Client: server.Client()}).Search(context.Background(), "示例番剧")
	if err != nil || len(got) != 1 || !strings.Contains(got[0].Title, "简中") {
		t.Fatal("provider pre-filtered a Chinese release", got, err)
	}
}
