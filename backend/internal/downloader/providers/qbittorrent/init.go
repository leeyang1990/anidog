package qbittorrent

import (
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/downloader"
)

func init() {
	downloader.Register("qbittorrent", func(cfg *config.Config) (downloader.Downloader, error) {
		return NewProvider(cfg)
	})
}
