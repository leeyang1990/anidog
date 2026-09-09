package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const desktopDirName = "AniDog"

// DesktopPaths 描述桌面版真正落盘的位置，供日志、原生能力和测试使用。
type DesktopPaths struct {
	DataRoot  string
	MediaRoot string
	Database  string
	BTState   string
}

// LoadDesktop 在普通配置之上施加桌面版必须的本地持久化约束。
func LoadDesktop() (*Config, DesktopPaths, error) {
	cfg := Load()
	paths, err := ConfigureDesktop(cfg, os.Getenv("ANIDOG_DESKTOP_DATA_DIR"), os.Getenv("ANIDOG_DESKTOP_MEDIA_ROOT"))
	return cfg, paths, err
}

// ConfigureDesktop 将运行时固定为 SQLite + 内嵌 BT，并为首次启动创建安全的本地目录。
// dataRoot 和 mediaRoot 留空时使用系统标准目录。
func ConfigureDesktop(cfg *Config, dataRoot, mediaRoot string) (DesktopPaths, error) {
	if cfg == nil {
		return DesktopPaths{}, fmt.Errorf("配置不能为空")
	}
	if strings.TrimSpace(dataRoot) == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return DesktopPaths{}, fmt.Errorf("读取用户配置目录: %w", err)
		}
		dataRoot = filepath.Join(base, desktopDirName)
	}
	dataRoot, err := filepath.Abs(dataRoot)
	if err != nil {
		return DesktopPaths{}, fmt.Errorf("解析桌面数据目录: %w", err)
	}

	if strings.TrimSpace(mediaRoot) == "" {
		mediaRoot = strings.TrimSpace(cfg.MediaRoot)
	}
	if mediaRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return DesktopPaths{}, fmt.Errorf("读取用户目录: %w", err)
		}
		mediaRoot = filepath.Join(home, "Downloads", desktopDirName)
	}
	mediaRoot, err = filepath.Abs(mediaRoot)
	if err != nil {
		return DesktopPaths{}, fmt.Errorf("解析桌面下载目录: %w", err)
	}

	btState := filepath.Join(dataRoot, "torrent")
	for _, dir := range []string{dataRoot, mediaRoot, btState} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return DesktopPaths{}, fmt.Errorf("创建桌面目录 %s: %w", dir, err)
		}
	}
	secret, err := loadOrCreateDesktopSecret(filepath.Join(dataRoot, "secret.key"))
	if err != nil {
		return DesktopPaths{}, err
	}

	databasePath := filepath.Join(dataRoot, "anidog.db")
	cfg.DatabaseURL = "sqlite://" + databasePath
	cfg.SecretKey = secret
	cfg.DownloaderType = "embedded"
	cfg.BTStateDir = btState
	cfg.BTListenPort = 0 // 桌面端默认使用空闲端口，避免和 Docker 或其他客户端冲突。
	if raw := strings.TrimSpace(os.Getenv("ANIDOG_DESKTOP_BT_LISTEN_PORT")); raw != "" {
		port, parseErr := strconv.Atoi(raw)
		if parseErr != nil || port < 0 || port > 65535 {
			return DesktopPaths{}, fmt.Errorf("ANIDOG_DESKTOP_BT_LISTEN_PORT 无效: %q", raw)
		}
		cfg.BTListenPort = port
	}
	cfg.MediaRoot = mediaRoot
	cfg.StreamDownloadDir = mediaRoot

	return DesktopPaths{
		DataRoot:  dataRoot,
		MediaRoot: mediaRoot,
		Database:  databasePath,
		BTState:   btState,
	}, nil
}

func loadOrCreateDesktopSecret(path string) (string, error) {
	if raw, err := os.ReadFile(path); err == nil {
		secret := strings.TrimSpace(string(raw))
		if len(secret) >= 32 {
			return secret, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("读取桌面密钥: %w", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成桌面密钥: %w", err)
	}
	secret := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("保存桌面密钥: %w", err)
	}
	return secret, nil
}
