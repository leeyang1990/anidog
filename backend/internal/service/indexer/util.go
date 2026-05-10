package indexer

import (
	"strconv"
	"strings"
)

// parseHumanSize 把 "849.25 MB" / "1.5 GB" / "500KB" 等转成字节数
func parseHumanSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// 找到数字末尾
	numEnd := 0
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			numEnd = i + 1
			continue
		}
		break
	}
	if numEnd == 0 {
		return 0
	}
	numStr := s[:numEnd]
	unit := strings.ToUpper(strings.TrimSpace(s[numEnd:]))

	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	mult := int64(1)
	switch unit {
	case "B":
		mult = 1
	case "KB", "K", "KIB":
		mult = 1024
	case "MB", "M", "MIB":
		mult = 1024 * 1024
	case "GB", "G", "GIB":
		mult = 1024 * 1024 * 1024
	case "TB", "T", "TIB":
		mult = 1024 * 1024 * 1024 * 1024
	default:
		mult = 1024 * 1024 // 默认 MB
	}
	return int64(f * float64(mult))
}
