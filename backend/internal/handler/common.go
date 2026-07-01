package handler

import (
	"context"
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
	"gorm.io/gorm"
)

// serviceStart 是 AniDog 进程的启动时刻，用于算"服务运行时长"。
// 在包初始化时取一次（import 阶段就执行），近似 = 进程启动时间。
var serviceStart = time.Now()

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

// round1 把百分比保留 1 位小数（11.747266 → 11.7），避免前端显示一长串。
func round1(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}

// humanizeDuration 把秒数格式化成"N天 N小时 N分钟"。
func humanizeDuration(sec uint64) string {
	days := sec / 86400
	hours := (sec % 86400) / 3600
	minutes := (sec % 3600) / 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%d小时 %d分钟", hours, minutes)
	default:
		return fmt.Sprintf("%d分钟", minutes)
	}
}

// SystemInfoDeps 是 getSystemInfo 需要的外部依赖：DB 连接池 + 下载器健康探针。
// 两者都可空 —— 空时对应字段返回"未知/离线"，不影响其余指标。
type SystemInfoDeps struct {
	DB        *gorm.DB
	QBitPing  func(ctx context.Context) (online bool, version string) // 可空
}

// getSystemInfo gathers system information using gopsutil + 运行时 + DB + qBit。
func getSystemInfo(version string, deps SystemInfoDeps) gin.H {
	cpuUsage, err := cpu.Percent(1*time.Second, false)
	if err != nil || len(cpuUsage) == 0 {
		zap.L().Warn("获取 CPU 使用率失败", zap.Error(err))
		cpuUsage = []float64{0}
	}

	memInfo := gin.H{}
	var memPercent float64
	if vmStat, err := mem.VirtualMemory(); err != nil {
		zap.L().Warn("获取内存信息失败", zap.Error(err))
	} else {
		memPercent = vmStat.UsedPercent
		memInfo = gin.H{
			"total":        vmStat.Total,
			"used":         vmStat.Used,
			"available":    vmStat.Available,
			"used_percent": round1(vmStat.UsedPercent),
		}
	}

	diskInfo := gin.H{}
	var diskPercent float64
	if diskStat, err := disk.Usage("/"); err != nil {
		zap.L().Warn("获取磁盘信息失败", zap.Error(err))
	} else {
		diskPercent = diskStat.UsedPercent
		diskInfo = gin.H{
			"total":        diskStat.Total,
			"used":         diskStat.Used,
			"free":         diskStat.Free,
			"used_percent": round1(diskStat.UsedPercent),
		}
	}

	// 服务运行时长（AniDog 进程自己），比宿主机 uptime 更有意义
	serviceUptime := humanizeDuration(uint64(time.Since(serviceStart).Seconds()))

	// 宿主机 uptime 作为辅助字段保留
	hostUptime := "未知"
	if hostStat, err := host.Info(); err != nil {
		zap.L().Warn("获取主机信息失败", zap.Error(err))
	} else {
		hostUptime = humanizeDuration(hostStat.Uptime)
	}

	// 运行时内存（Go 进程自己占的，区别于系统内存）
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// DB 连接池状态
	dbInfo := gin.H{"connected": false}
	if deps.DB != nil {
		if sqlDB, err := deps.DB.DB(); err == nil {
			if pingErr := sqlDB.Ping(); pingErr == nil {
				st := sqlDB.Stats()
				dbInfo = gin.H{
					"connected":     true,
					"open":          st.OpenConnections,
					"in_use":        st.InUse,
					"idle":          st.Idle,
					"wait_count":    st.WaitCount,
					"max_open":      st.MaxOpenConnections,
				}
			}
		}
	}

	// qBittorrent 状态
	qbitInfo := gin.H{"online": false}
	if deps.QBitPing != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		online, qVer := deps.QBitPing(ctx)
		cancel()
		qbitInfo = gin.H{"online": online, "version": qVer}
	}

	return gin.H{
		"version": version,
		// 主显示用"服务运行时长"
		"uptime":     serviceUptime,
		"hostUptime": hostUptime,
		// 百分比统一保留 1 位
		"cpuUsage":    round1(cpuUsage[0]),
		"memoryUsage": round1(memPercent),
		"diskUsage":   round1(diskPercent),
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"goVersion":   runtime.Version(),
		"cpuCores":    runtime.NumCPU(),
		"goroutines":  runtime.NumGoroutine(),
		"goMemory": gin.H{
			"alloc":       ms.Alloc,       // 当前堆上分配且仍在用的字节
			"sys":         ms.Sys,         // 向 OS 申请的总字节
			"num_gc":      ms.NumGC,       // GC 次数
		},
		"memory":    memInfo,
		"disk":      diskInfo,
		"database":  dbInfo,
		"qbittorrent": qbitInfo,
		"timestamp": time.Now(),
	}
}
