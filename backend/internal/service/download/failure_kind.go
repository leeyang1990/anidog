// Package download · 失败分类与重试退避
//
// 这里集中收"什么样的错算 transient（可重试）vs permanent（永久放弃）"，
// 以及"transient 时下一次重试该多久后"。集中之后:
//   - execute() 在 Execute() 返回 err 时，调一次 classifyError → 写 failure_kind/last_error/next_retry_at
//   - RetryFailedJob 在到点时按 retry_count 决定要不要继续重试
//   - isDuplicate 用 failure_kind 判断该不该让 transient 失败的集"立刻再来一次"
//
// 设计点：
//  1. 默认是 permanent —— 不认识的错误宁可被人手动救，也不要疯狂重试浪费配额。
//  2. 流媒体（StreamExecutor + ffmpeg）的几个签名/链路错最常见，单独枚举。
//  3. BT 死种是另一个分类入口（qbit_sync.abandonDeadTorrent），不走这里 ——
//     那条路径只发生在我们主动放弃种子时，详见 qbit_sync.go。
package download

import (
	"strings"
	"time"

	"github.com/anidog/anidog-go/internal/model"
)

// classifyError 把一条下载错误归类为 transient / permanent，并附带建议的下次重试时间。
// 调用方拿到结果后写回 download 表，由 RetryFailedJob 调度真正的重试。
//
// 返回的 nextDelay 是相对当前的退避时间；调用方应叠加 time.Now()。
// retryCount 是"已经重试过的次数"——首次失败传 0。
func classifyError(err error, retryCount int) (kind string, nextDelay time.Duration) {
	if err == nil {
		return "", 0
	}
	// retryCount 表示已经消耗的重试次数。第 3 次重试仍失败后必须收敛为
	// permanent；这个判断要放在错误 marker 之前，否则已识别的网络错误会
	// 返回 transient + 0 延迟，形成永远不会再调度、语义却仍可重试的僵尸行。
	if retryCount >= 3 {
		return model.FailureKindPermanent, 0
	}
	msg := strings.ToLower(err.Error())

	// ---- transient：流媒体源的临时故障 ----
	// 1. m3u8 签名/token 过期：ffmpeg "IO error: End of file" / "End of file"
	// 2. HTTP 4xx 鉴权：403 Forbidden / 401 Unauthorized
	// 3. 临时网络抖动：connection refused / connection reset / timeout / tls handshake
	// 4. CDN 502/503/504：服务暂时不可用，重试基本能下到
	transientMarkers := []string{
		"end of file",
		"io error",
		"403 forbidden",
		"http error 403",
		"401 unauthorized",
		"http error 401",
		"connection refused",
		"connection reset",
		"i/o timeout",
		"deadline exceeded",
		"tls handshake",
		"502 bad gateway",
		"503 service unavailable",
		"504 gateway timeout",
		"http error 5",
		// rod 拉详情页失败的几个典型现象
		"context deadline exceeded",
		"net::err_",
	}
	for _, m := range transientMarkers {
		if strings.Contains(msg, m) {
			return model.FailureKindTransient, backoff(retryCount)
		}
	}

	// ---- permanent：明显不该重试 ----
	// 磁盘满、权限错、ffmpeg 不支持的格式、URL 非法
	permanentMarkers := []string{
		"no space left",
		"permission denied",
		"read-only file system",
		"invalid data found",
		"invalid argument",
		"unsupported codec",
		"unknown format",
		"unknown encoder",
	}
	for _, m := range permanentMarkers {
		if strings.Contains(msg, m) {
			return model.FailureKindPermanent, 0
		}
	}

	// 其余视为 transient 试一两次 —— 多数 ffmpeg/网络错都属于"暂时性"。
	return model.FailureKindTransient, backoff(retryCount)
}

// backoff —— transient 重试的等待时间，按重试次数递增。
//
//	第 0 次重试（即首次失败后第一次重试）：10 分钟
//	第 1 次重试：1 小时
//	第 2 次重试：6 小时
//	≥ 3 次：返回 0，并由 RetryFailedJob 跳过（最多重试 3 次）
//
// 退避节奏的逻辑：流媒体 m3u8 签名一般 5-15 分钟过期，所以 10min 后再试，
// 大概率拿到的是新签名的 URL；第二次还失败说明源本身坏了，拉长到 1h；
// 6h 是 BT 死种通常会出现新副本的时间窗。
func backoff(retryCount int) time.Duration {
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
	return s[:max] + "...(truncated)"
}
