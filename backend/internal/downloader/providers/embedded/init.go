package embedded

import (
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/downloader"
)

func init() {
	downloader.Register("embedded", func(cfg *config.Config) (downloader.TorrentEngine, error) {
		return New(cfg)
	})
}
