package downloader

import (
	"sync"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/config"
)

// ProviderFactory 下载器工厂函数类型
type ProviderFactory func(cfg *config.Config) (Downloader, error)

// Registry 下载器注册表
type Registry struct {
	mu       sync.RWMutex
	factories map[string]ProviderFactory
}

var (
	globalRegistry = &Registry{
		factories: make(map[string]ProviderFactory),
	}
)

// Register 注册下载器工厂
func Register(name string, factory ProviderFactory) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.factories[name] = factory
	zap.L().Info("注册下载器", zap.String("name", name))
}

// Create 创建下载器实例
func Create(downloaderType string, cfg *config.Config) (Downloader, error) {
	globalRegistry.mu.RLock()
	factory, ok := globalRegistry.factories[downloaderType]
	globalRegistry.mu.RUnlock()

	if !ok {
		return nil, &DownloaderError{Type: Unsupported, Name: downloaderType}
	}

	return factory(cfg)
}

// ListProviders 列出所有已注册的下载器
func ListProviders() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	providers := make([]string, 0, len(globalRegistry.factories))
	for name := range globalRegistry.factories {
		providers = append(providers, name)
	}
	return providers
}

// DownloaderError 下载器错误
type DownloaderError struct {
	Type   DownloaderErrorType
	Name   string
}

// DownloaderErrorType 错误类型
type DownloaderErrorType int

const (
	Unsupported DownloaderErrorType = iota
	InvalidConfig
	ConnectionFailed
)

func (e *DownloaderError) Error() string {
	switch e.Type {
	case Unsupported:
		return "不支持的下载器类型: " + e.Name
	case InvalidConfig:
		return "下载器配置无效: " + e.Name
	case ConnectionFailed:
		return "下载器连接失败: " + e.Name
	default:
		return "未知错误: " + e.Name
	}
}
