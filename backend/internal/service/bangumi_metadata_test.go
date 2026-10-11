package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
)

func TestMainEpisodeCountDoesNotUseAllChapterCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1,"name":"test","eps":12,"total_episodes":24}`)
	}))
	defer srv.Close()
	svc := NewBangumiService(&config.Config{BangumiAPIURL: srv.URL}, srv.Client())
	detail, err := svc.GetAnimeDetail(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if detail.EpsCount != 12 || detail.TotalEpisodes != 24 {
		t.Fatalf("main=%d all=%d", detail.EpsCount, detail.TotalEpisodes)
	}
}
