package download

import (
	"testing"

	"github.com/anidog/anidog-go/internal/service/mediacheck"
)

func TestHLSPlaylistSizeIsNotAnEpisodeReservation(t *testing.T) {
	var m mediacheck.Media
	m.Format.Name, m.Format.Size, m.Format.Duration = "hls", "96", "1455.6"
	if got := estimateStreamBytes(m, 2*gib); got != 2*gib {
		t.Fatalf("reserved %d bytes for a full episode", got)
	}
	m.Format.BitRate = "2000000"
	if got := estimateStreamBytes(m, 2*gib); got < 400_000_000 {
		t.Fatal(got)
	}
	m.Format.Name, m.Format.Size = "mov,mp4,m4a,3gp,3g2,mj2", "303818209"
	if got := estimateStreamBytes(m, 2*gib); got != 334200029 {
		t.Fatal(got)
	}
}
