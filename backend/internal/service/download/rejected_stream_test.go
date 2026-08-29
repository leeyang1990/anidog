package download

import (
	"context"
	"testing"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

func TestTriggerRejectedStreamRecovery(t *testing.T) {
	called := make(chan struct {
		animeID uint
		episode int
	}, 1)
	service := &Service{}
	service.SetRejectedStreamHandler(func(_ context.Context, animeID uint, episode int) {
		called <- struct {
			animeID uint
			episode int
		}{animeID: animeID, episode: episode}
	})
	animeID, episode := uint(10), 3
	service.triggerRejectedStreamRecovery(&Task{
		DownloadType:  model.DownloadTypeStream,
		AnimeID:       &animeID,
		EpisodeNumber: &episode,
	})

	select {
	case got := <-called:
		if got.animeID != animeID || got.episode != episode {
			t.Fatalf("got anime=%d episode=%d, want anime=%d episode=%d",
				got.animeID, got.episode, animeID, episode)
		}
	case <-time.After(time.Second):
		t.Fatal("rejected stream recovery callback was not invoked")
	}
}

func TestTriggerRejectedStreamRecoveryIgnoresTorrent(t *testing.T) {
	called := make(chan struct{}, 1)
	service := &Service{}
	service.SetRejectedStreamHandler(func(context.Context, uint, int) { called <- struct{}{} })
	animeID, episode := uint(10), 3
	service.triggerRejectedStreamRecovery(&Task{
		DownloadType:  model.DownloadTypeTorrent,
		AnimeID:       &animeID,
		EpisodeNumber: &episode,
	})

	select {
	case <-called:
		t.Fatal("torrent rejection must not invoke stream recovery callback")
	case <-time.After(50 * time.Millisecond):
	}
}
