package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/anidog/anidog-go/internal/application"
	"github.com/anidog/anidog-go/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

const desktopVersion = "0.3.0"

func main() {
	// 发布构建仍可通过 -ldflags 覆盖；本地 Wails 构建至少与应用包版本一致。
	if config.Version == "dev" {
		config.Version = desktopVersion
	}
	cfg, paths, err := config.LoadDesktop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "AniDog 桌面配置初始化失败:", err)
		os.Exit(1)
	}

	appRuntime, err := application.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "AniDog 桌面运行时初始化失败:", err)
		os.Exit(1)
	}
	bridge := NewDesktopBridge(paths)

	err = wails.Run(&options.App{
		Title:                    "AniDog",
		Width:                    1440,
		Height:                   900,
		MinWidth:                 1024,
		MinHeight:                680,
		BackgroundColour:         options.NewRGB(247, 244, 235),
		EnableDefaultContextMenu: false,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: appRuntime.Handler,
		},
		OnStartup: bridge.startup,
		OnShutdown: func(ctx context.Context) {
			_ = appRuntime.Close(ctx)
		},
		Bind: []interface{}{bridge},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.anidog.desktop",
		},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarDefault(),
			Appearance: mac.DefaultAppearance,
			About: &mac.AboutInfo{
				Title:   "AniDog",
				Message: "本地运行的番剧自动下载管理器",
			},
		},
	})
	if err != nil {
		_ = appRuntime.Close(context.Background())
		fmt.Fprintln(os.Stderr, "AniDog 桌面端启动失败:", err)
		os.Exit(1)
	}
}
