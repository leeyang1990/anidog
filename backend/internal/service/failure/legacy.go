package failure

import (
	"github.com/anidog/anidog-go/internal/model"
	"strings"
)

func inconclusiveSubtitle(message string) bool {
	return strings.Contains(message, "无法确认成片满足字幕") || strings.Contains(message, "抽样字幕证据不足")
}

// Keep historical text interpretation here, not in schedulers or SQL queries.
// New producers must supply a code at the stage that knows what failed.
func legacy(message string) Facts {
	if inconclusiveSubtitle(message) {
		return facts(SubtitleUnknown, Subtitles)
	}
	msg := strings.ToLower(message)
	// A failed probe has not established any content or preference mismatch.
	// Older releases persisted these network/tool errors as rejected candidates.
	if strings.Contains(msg, "流媒体预检探测失败") {
		for _, marker := range []string{"server returned", "http error", "connection refused", "connection reset", "timed out", "timeout", "deadline exceeded", "no route to host", "network is unreachable", "unable to resolve", "failed to resolve"} {
			if strings.Contains(msg, marker) {
				return facts(SourceUnavailable, Probe)
			}
		}
		return facts(InspectionFailed, Probe)
	}
	for _, marker := range []string{"no space left", "permission denied", "read-only file system", "invalid data found", "invalid argument", "unsupported codec", "unknown format", "unknown encoder"} {
		if strings.Contains(msg, marker) {
			return facts(StorageUnavailable, "")
		}
	}
	if strings.Contains(msg, "字幕检测暂时不可用") {
		return facts(InspectionFailed, Subtitles)
	}
	if strings.Contains(msg, "播放器已报错") {
		return facts(PlayerUnavailable, Player)
	}
	for _, marker := range []string{"流媒体预检不符合要求", "流媒体候选无效", "视频时长过短", "视频疑似下载截断", "季度不匹配", "季数不匹配", "错误选中", "season mismatch", "wrong season", "集数不匹配", "剧集不匹配", "episode mismatch", "qbittorrent 中不存在对应任务", "bt 引擎中不存在对应任务"} {
		if strings.Contains(msg, marker) {
			return facts(InvalidMedia, "")
		}
	}
	return Facts{Kind: model.FailureKindTransient, AvailabilityFailure: true}
}
