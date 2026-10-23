// Package download centralizes failure classification and bounded retry delays.
package download

import (
	"time"
	"unicode/utf8"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/failure"
)

// classifyError distinguishes local repair, candidate replacement and transient
// execution failures. Only actual executions consume the rapid retry budget.
func classifyError(err error, retryCount int) (string, time.Duration) {
	if err == nil {
		return "", 0
	}
	facts := failure.Describe(err)
	if facts.Kind != model.FailureKindTransient {
		return facts.Kind, 0
	}
	if retryCount >= 3 {
		return model.FailureKindExhausted, 0
	}
	// Unknown ffmpeg/browser errors receive the same bounded budget as network
	// failures, instead of an unbounded loop or immediate permanent abandonment.
	return model.FailureKindTransient, RetryDelay(retryCount)
}

func failureFields(err error, retryCount int) (map[string]interface{}, string, time.Duration) {
	facts := failure.Describe(err)
	kind, delay := classifyError(err, retryCount)
	fields := map[string]interface{}{
		"failure_kind": kind, "failure_code": string(facts.Code), "failure_stage": string(facts.Stage),
		"last_error": truncateError(err), "next_retry_at": nil,
	}
	if delay > 0 {
		next := time.Now().Add(delay)
		fields["next_retry_at"] = &next
	}
	return fields, kind, delay
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
