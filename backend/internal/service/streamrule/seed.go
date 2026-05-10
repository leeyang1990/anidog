package streamrule

import (
	"context"
	"embed"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/model"
)

//go:embed default_rules/*.json
var defaultRulesFS embed.FS

// SeedDefaultRules 导入内置的 Kazumi 默认规则（按名称判重，已存在的跳过）。
// 来源: https://github.com/Predidit/KazumiRules (MIT License)
func (s *Service) SeedDefaultRules(ctx context.Context) error {
	entries, err := defaultRulesFS.ReadDir("default_rules")
	if err != nil {
		zap.L().Warn("无法读取内置规则目录", zap.Error(err))
		return nil
	}

	// 收集已有规则名，避免重复导入
	var existing []string
	if err := s.db.WithContext(ctx).Model(&model.StreamRule{}).Pluck("name", &existing).Error; err != nil {
		return err
	}
	existingSet := make(map[string]bool, len(existing))
	for _, n := range existing {
		existingSet[n] = true
	}

	var rawRules []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := defaultRulesFS.ReadFile("default_rules/" + entry.Name())
		if err != nil {
			zap.L().Warn("读取规则文件失败", zap.String("file", entry.Name()), zap.Error(err))
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			zap.L().Warn("解析规则 JSON 失败", zap.String("file", entry.Name()), zap.Error(err))
			continue
		}
		// 跳过已存在的规则
		if name, ok := raw["name"].(string); ok && existingSet[name] {
			continue
		}
		rawRules = append(rawRules, raw)
	}

	if len(rawRules) == 0 {
		return nil
	}

	result := s.ImportKazumiRules(ctx, rawRules)
	zap.L().Info("已导入内置 Kazumi 规则",
		zap.Int("total", result.Total),
		zap.Int("imported", result.Imported),
		zap.Int("failed", result.Failed))
	return nil
}
