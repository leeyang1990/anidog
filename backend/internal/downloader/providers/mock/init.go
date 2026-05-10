package mock

import (
	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/downloader"
)

func init() {
	downloader.Register("mock", func(cfg *config.Config) (downloader.Downloader, error) {
		return NewProvider(cfg)
	})
}
