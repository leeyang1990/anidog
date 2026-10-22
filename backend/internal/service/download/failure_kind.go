// Package download centralizes failure classification and bounded retry delays.
package download

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anidog/anidog-go/internal/model"
)

// classifyError distinguishes local repair, candidate replacement and transient
// execution failures. Only actual executions consume the rapid retry budget.
func classifyError(err error, retryCount int) (string, time.Duration) {
	if err == nil {
		return "", 0
	}
	msg := strings.ToLower(err.Error())
	// Local failures do not become remote-source exhaustion merely because the
	// attempt budget ran out. Keep their actionable cause and longer cooldown.
	for _, marker := range []string{
		"no space left", "permission denied", "read-only file system",
		"invalid data found", "invalid argument", "unsupported codec",
		"unknown format", "unknown encoder",
	} {
		if strings.Contains(msg, marker) {
			return model.FailureKindPermanent, 0
		}
	}
	if isRejectedCandidateError(msg) {
		return model.FailureKindRejected, 0
	}
	if retryCount >= 3 {
		return model.FailureKindExhausted, 0
	}
	// Unknown ffmpeg/browser errors receive the same bounded budget as network
	// failures, instead of an unbounded loop or immediate permanent abandonment.
	return model.FailureKindTransient, RetryDelay(retryCount)
}

// isRejectedCandidateError 判断错误是否只说明“这一个候选不适合当前剧集”。
// 这类错误与网络抖动不同：重复提交同一个候选没有意义，但应允许编排器
// 立刻尝试下一候选，所以使用 rejected，而不是 transient/permanent。
func isRejectedCandidateError(msg string) bool {
	markers := []string{
		"播放器已报错",
		"流媒体预检不符合要求",
		"流媒体候选无效",
		"视频时长过短",
		"视频疑似下载截断",
		"季度不匹配",
		"季数不匹配",
		"错误选中",
		"season mismatch",
		"wrong season",
		"集数不匹配",
		"剧集不匹配",
		"episode mismatch",
		"qbittorrent 中不存在对应任务",
		"bt 引擎中不存在对应任务",
	}
	for _, marker := range markers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// RetryDelay is the shared rapid-retry delay for execution and candidate selection.
//
//	第 0 次重试（即首次失败后第一次重试）：10 分钟
//	第 1 次重试：1 小时
//	第 2 次重试：6 小时
//	≥ 3 次：返回 0，并由 RetryFailedJob 跳过（最多重试 3 次）
//
// 退避节奏的逻辑：流媒体 m3u8 签名一般 5-15 分钟过期，所以 10min 后再试，
// 大概率拿到的是新签名的 URL；第二次还失败说明源本身坏了，拉长到 1h；
// 6h 是 BT 死种通常会出现新副本的时间窗。
func RetryDelay(retryCount int) time.Duration {
	switch retryCount {
	case 0:
		return 10 * time.Minute
	case 1:
		return 1 * time.Hour
	case 2:
		return 6 * time.Hour
	default:
		return 0
	}
}

// truncateError —— last_error 字段截断到 1 KiB，避免 ffmpeg 整段日志撑爆 TEXT 列。
func truncateError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	const max = 1024
	if len(s) <= max {
		return s
	}
	end := max
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "...(truncated)"
}
