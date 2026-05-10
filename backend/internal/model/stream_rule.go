package model

import (
	"encoding/json"
	"strconv"
	"time"
)

// StreamRule 流媒体规则数据库模型
type StreamRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"index;not null" json:"name"`
	DisplayName *string `json:"display_name"`
	Version   string    `gorm:"default:'1.0'" json:"version"`
	APILevel  int       `gorm:"default:6" json:"api_level"`
	Enabled   bool      `gorm:"index;default:true" json:"enabled"`

	// 站点配置
	BaseURL    string `gorm:"not null" json:"base_url"`
	SearchURL  string `gorm:"not null" json:"search_url"`
	UsePost    bool   `gorm:"default:false" json:"use_post"`
	UserAgent  *string `json:"user_agent"`
	Referer    *string `json:"referer"`
	UseWebview bool   `gorm:"default:false" json:"use_webview"`
	MultiSources bool  `gorm:"default:true" json:"multi_sources"`

	// XPath 选择器
	SearchListXPath   string  `gorm:"not null" json:"search_list_xpath"`
	SearchNameXPath   string  `gorm:"not null" json:"search_name_xpath"`
	SearchResultXPath string  `gorm:"not null" json:"search_result_xpath"`
	ChapterRoadsXPath *string `json:"chapter_roads_xpath"`
	ChapterResultXPath string `gorm:"not null" json:"chapter_result_xpath"`

	// 额外配置 (JSON)
	AntiCrawlerConfig *string `json:"anti_crawler_config"`
	Headers           *string `json:"headers"`
	Cookies           *string `json:"cookies"`

	// 规则健康状态（由 SourceHealthService 聚合计算）
	// 取值: "" healthy degraded broken
	HealthStatus *string    `json:"health_status" gorm:"index"`
	HealthNote   *string    `json:"health_note"`
	HealthAt     *time.Time `json:"health_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (StreamRule) TableName() string { return "streamrule" }

// StreamRuleCreate 创建流媒体规则请求
type StreamRuleCreate struct {
	Name              string  `json:"name" binding:"required"`
	DisplayName       *string `json:"display_name"`
	Version           *string `json:"version"`
	APILevel          *int    `json:"api_level"`
	BaseURL           string  `json:"base_url" binding:"required"`
	SearchURL         string  `json:"search_url" binding:"required"`
	UsePost           *bool   `json:"use_post"`
	UserAgent         *string `json:"user_agent"`
	Referer           *string `json:"referer"`
	UseWebview        *bool   `json:"use_webview"`
	MultiSources      *bool   `json:"multi_sources"`
	SearchListXPath   string  `json:"search_list_xpath" binding:"required"`
	SearchNameXPath   string  `json:"search_name_xpath" binding:"required"`
	SearchResultXPath string  `json:"search_result_xpath" binding:"required"`
	ChapterRoadsXPath *string `json:"chapter_roads_xpath"`
	ChapterResultXPath string `json:"chapter_result_xpath" binding:"required"`
	AntiCrawlerConfig *string `json:"anti_crawler_config"`
	Headers           *string `json:"headers"`
	Cookies           *string `json:"cookies"`
}

// StreamRuleUpdate 更新流媒体规则请求
type StreamRuleUpdate struct {
	Name              *string `json:"name"`
	DisplayName       *string `json:"display_name"`
	Version           *string `json:"version"`
	APILevel          *int    `json:"api_level"`
	Enabled           *bool   `json:"enabled"`
	BaseURL           *string `json:"base_url"`
	SearchURL         *string `json:"search_url"`
	UsePost           *bool   `json:"use_post"`
	UserAgent         *string `json:"user_agent"`
	Referer           *string `json:"referer"`
	UseWebview        *bool   `json:"use_webview"`
	MultiSources      *bool   `json:"multi_sources"`
	SearchListXPath   *string `json:"search_list_xpath"`
	SearchNameXPath   *string `json:"search_name_xpath"`
	SearchResultXPath *string `json:"search_result_xpath"`
	ChapterRoadsXPath *string `json:"chapter_roads_xpath"`
	ChapterResultXPath *string `json:"chapter_result_xpath"`
	AntiCrawlerConfig *string `json:"anti_crawler_config"`
	Headers           *string `json:"headers"`
	Cookies           *string `json:"cookies"`
}

// kazumiFieldMap Kazumi 字段名 → StreamRule 字段名映射
var kazumiFieldMap = map[string]string{
	"api":              "api_level",
	"baseURL":          "base_url",
	"searchURL":        "search_url",
	"usePost":          "use_post",
	"userAgent":        "user_agent",
	"useWebview":       "use_webview",
	"useNativePlayer":  "", // 忽略
	"useLegacyParser":  "", // 忽略
	"adBlocker":        "", // 忽略
	"muliSources":      "multi_sources", // Kazumi 拼写为 muli
	"searchList":       "search_list_xpath",
	"searchName":       "search_name_xpath",
	"searchResult":     "search_result_xpath",
	"chapterRoads":     "chapter_roads_xpath",
	"chapterResult":    "chapter_result_xpath",
	"antiCrawlerConfig": "anti_crawler_config",
}

// MapKazumiRule 将 Kazumi JSON 格式映射为 StreamRule 字段
func MapKazumiRule(data map[string]interface{}) map[string]interface{} {
	mapped := make(map[string]interface{})
	for k, v := range data {
		if k == "type" {
			continue
		}
		targetKey, exists := kazumiFieldMap[k]
		if exists && targetKey == "" {
			continue // 显式忽略的字段
		}
		if targetKey != "" {
			if targetKey == "api_level" {
				if f, ok := toInt(v); ok {
					v = f
				} else {
					v = 6
				}
			}
			if targetKey == "anti_crawler_config" {
				if m, ok := v.(map[string]interface{}); ok {
					b, _ := json.Marshal(m)
					v = string(b)
				}
			}
			mapped[targetKey] = v
		} else {
			// 未映射的字段直接传递 (name, version, referer 等同名字段)
			mapped[k] = v
		}
	}
	return mapped
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case string:
		i, err := strconv.Atoi(val)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}
