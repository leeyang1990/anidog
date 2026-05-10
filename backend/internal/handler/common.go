package handler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
)

func generateStreamID() string {
	return uuid.New().String()[:8]
}

// toUserResponse converts a User model to a response map.
func toUserResponse(user model.User) gin.H {
	return gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"is_admin":   user.IsAdmin,
		"is_active":  user.IsActive,
		"created_at": user.CreatedAt,
	}
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page,omitempty"`
	Size  int         `json:"per_page,omitempty"`
}

// ErrorResponse 错误响应
func errorResponse(detail string) map[string]interface{} {
	return map[string]interface{}{"detail": detail}
}

// FormatFileSize 格式化文件大小
func FormatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// getSystemInfo gathers system information using gopsutil.
func getSystemInfo(version string) gin.H {
	cpuUsage, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		zap.L().Warn("获取 CPU 使用率失败", zap.Error(err))
		cpuUsage = []float64{0}
	}

	memInfo := gin.H{}
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		zap.L().Warn("获取内存信息失败", zap.Error(err))
	} else {
		memInfo = gin.H{
			"total":        vmStat.Total,
			"used":         vmStat.Used,
			"available":    vmStat.Available,
			"used_percent": vmStat.UsedPercent,
		}
	}

	diskInfo := gin.H{}
	diskStat, err := disk.Usage("/")
	if err != nil {
		zap.L().Warn("获取磁盘信息失败", zap.Error(err))
	} else {
		diskInfo = gin.H{
			"total":        diskStat.Total,
			"used":         diskStat.Used,
			"free":         diskStat.Free,
			"used_percent": diskStat.UsedPercent,
		}
	}

	uptimeStr := "未知"
	hostStat, err := host.Info()
	if err != nil {
		zap.L().Warn("获取主机信息失败", zap.Error(err))
	} else {
		uptimeSec := hostStat.Uptime
		days := uptimeSec / 86400
		hours := (uptimeSec % 86400) / 3600
		minutes := (uptimeSec % 3600) / 60
		switch {
		case days > 0:
			uptimeStr = fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)
		case hours > 0:
			uptimeStr = fmt.Sprintf("%d小时 %d分钟", hours, minutes)
		default:
			uptimeStr = fmt.Sprintf("%d分钟", minutes)
		}
	}

	return gin.H{
		"version":      version,
		"uptime":       uptimeStr,
		"cpuUsage":     cpuUsage[0],
		"memoryUsage":  vmStat.UsedPercent,
		"diskUsage":    diskStat.UsedPercent,
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"goVersion":    runtime.Version(),
		"cpuCores":     runtime.NumCPU(),
		"memory":       memInfo,
		"disk":         diskInfo,
		"timestamp":    time.Now(),
	}
}
