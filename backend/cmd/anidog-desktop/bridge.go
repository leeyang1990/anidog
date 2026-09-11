package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/desktopappearance"
)

type DesktopBridge struct {
	ctx   context.Context
	paths config.DesktopPaths
}

type DesktopInfo struct {
	Platform  string `json:"platform"`
	DataRoot  string `json:"data_root"`
	MediaRoot string `json:"media_root"`
	BTState   string `json:"bt_state"`
}

func NewDesktopBridge(paths config.DesktopPaths) *DesktopBridge {
	return &DesktopBridge{paths: paths}
}

func (b *DesktopBridge) startup(ctx context.Context) {
	b.ctx = ctx
}

func (b *DesktopBridge) Info() DesktopInfo {
	return DesktopInfo{
		Platform:  runtime.GOOS,
		DataRoot:  b.paths.DataRoot,
		MediaRoot: b.paths.MediaRoot,
		BTState:   b.paths.BTState,
	}
}

// ApplyAppearance runs only after the WebView is mounted; other platforms are no-ops.
func (b *DesktopBridge) ApplyAppearance(dark bool) desktopappearance.State {
	return desktopappearance.Apply(dark)
}

// SelectDirectory 使用系统原生目录选择器，替代浏览器版受媒体根目录限制的目录树。
func (b *DesktopBridge) SelectDirectory(current string) (string, error) {
	if b.ctx == nil {
		return "", fmt.Errorf("桌面窗口尚未就绪")
	}
	current = strings.TrimSpace(current)
	if info, err := os.Stat(current); err != nil || !info.IsDir() {
		current = b.paths.MediaRoot
	}
	return wailsRuntime.OpenDirectoryDialog(b.ctx, wailsRuntime.OpenDialogOptions{
		Title:                "选择 AniDog 下载目录",
		DefaultDirectory:     current,
		CanCreateDirectories: true,
		ResolvesAliases:      true,
	})
}

// OpenPath 在 Finder / Explorer / 文件管理器中打开下载位置。
func (b *DesktopBridge) OpenPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = b.paths.MediaRoot
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析路径: %w", err)
	}
	if info, statErr := os.Stat(abs); statErr != nil {
		return fmt.Errorf("路径不存在: %w", statErr)
	} else if !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", abs)
	case "windows":
		cmd = exec.Command("explorer", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开目录: %w", err)
	}
	return nil
}
