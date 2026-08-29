package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/config"
)

const (
	defaultMinStreamDurationSeconds = 100
	minCompletedDurationRatio       = 0.8
)

type videoProbeResult struct {
	CodecName string
	Duration  float64
	Size      int64
}

type ffprobeVideoOutput struct {
	Streams []struct {
		CodecName string `json:"codec_name"`
		CodecType string `json:"codec_type"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

var (
	outTimeMsRe = regexp.MustCompile(`out_time_ms=(\d+)`)
	timeRe      = regexp.MustCompile(`time=(\d+):(\d+):(\d+\.?\d*)`)
	sizeRe      = regexp.MustCompile(`(?:total_size|size)=(\d+)`)
)

// M3U8Downloader ffmpeg 下载器
type M3U8Downloader struct {
	cfg         *config.Config
	activeProcs sync.Map // taskID → *exec.Cmd
	semaphore   chan struct{}
}

func NewM3U8Downloader(cfg *config.Config) *M3U8Downloader {
	maxConcurrent := cfg.StreamMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &M3U8Downloader{
		cfg:       cfg,
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

// Download 下载 M3U8/MP4 视频
func (d *M3U8Downloader) Download(ctx context.Context, taskID, videoURL, outputPath, videoType, referer string, progressCB func(progress float64, downloadedBytes int64)) (string, error) {
	// 获取信号量
	select {
	case d.semaphore <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-d.semaphore }()

	// 获取总时长（用于进度计算）
	totalDuration := d.getM3U8Duration(ctx, videoURL, referer)

	cmd := d.buildFFmpegCmd(videoURL, outputPath, videoType, referer)
	d.activeProcs.Store(taskID, cmd)
	defer d.activeProcs.Delete(taskID)

	// 获取 stderr 用于进度解析
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("获取 stderr 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动 ffmpeg 失败: %w", err)
	}

	// 读取 stderr 解析进度
	var lastLines []string
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// 保留最后 20 行用于错误诊断
		lastLines = append(lastLines, line)
		if len(lastLines) > 20 {
			lastLines = lastLines[1:]
		}
		if progressCB != nil {
			pct, bytes, ok := d.parseFFmpegProgress(line, totalDuration)
			if ok {
				progressCB(pct, bytes)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		zap.L().Error("ffmpeg 下载失败",
			zap.String("url", videoURL),
			zap.String("output", outputPath),
			zap.Strings("stderr_tail", lastLines),
			zap.Error(err))
		return "", fmt.Errorf("ffmpeg 退出: %w", err)
	}

	// 下载成功后校验编码和时长。只检查“存在视频流”会把广告、反盗链占位片
	// 或提前结束的 HLS 当作完整剧集，因此必须在竞速仲裁前拦截。
	probe, err := d.validateVideo(outputPath, totalDuration)
	if err != nil {
		_ = os.Remove(outputPath)
		return "", fmt.Errorf("流媒体候选无效：%w", err)
	}

	zap.L().Info("ffmpeg 下载完成",
		zap.String("output", outputPath),
		zap.Float64("duration_seconds", probe.Duration),
		zap.String("codec", probe.CodecName))
	return outputPath, nil
}

// validateVideo 用 ffprobe 检查输出文件是否为可接受的完整视频。
// sourceDuration 是下载前从源地址探测到的时长；可用时还会防止成片被截断。
func (d *M3U8Downloader) validateVideo(path string, sourceDuration float64) (videoProbeResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return videoProbeResult{}, fmt.Errorf("读取成片失败: %w", err)
	}

	cmd := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "stream=codec_name,codec_type:format=duration",
		"-of", "json",
		path)
	out, err := cmd.Output()
	if err != nil {
		return videoProbeResult{}, fmt.Errorf("ffprobe 无法解析成片: %w", err)
	}

	var parsed ffprobeVideoOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return videoProbeResult{}, fmt.Errorf("解析 ffprobe 结果失败: %w", err)
	}
	codec := ""
	for _, stream := range parsed.Streams {
		if stream.CodecType == "video" {
			codec = strings.TrimSpace(strings.ToLower(stream.CodecName))
			break
		}
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(parsed.Format.Duration), 64)
	if err != nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return videoProbeResult{}, fmt.Errorf("无法获得有效视频时长: %q", parsed.Format.Duration)
	}
	probe := videoProbeResult{CodecName: codec, Duration: duration, Size: info.Size()}
	if err := validateVideoProbe(probe, sourceDuration, d.minDurationSeconds()); err != nil {
		return videoProbeResult{}, err
	}
	return probe, nil
}

func validateVideoProbe(probe videoProbeResult, sourceDuration float64, minimumSeconds int) error {
	if probe.Size < 100*1024 {
		return fmt.Errorf("视频文件过小: %d 字节", probe.Size)
	}
	// 真实视频编码：h264, h265/hevc, av1, vp9 等；png/mjpeg/jpeg 通常是
	// 反盗链返回的占位图片序列。
	codec := strings.TrimSpace(strings.ToLower(probe.CodecName))
	if codec == "" || codec == "png" || codec == "mjpeg" || codec == "jpeg" {
		return fmt.Errorf("未检测到有效视频编码: %q", codec)
	}
	if probe.Duration <= 0 || math.IsNaN(probe.Duration) || math.IsInf(probe.Duration, 0) {
		return fmt.Errorf("无法获得有效视频时长")
	}
	if minimumSeconds > 0 && probe.Duration < float64(minimumSeconds) {
		return fmt.Errorf("视频时长过短: %.2f 秒，最低要求 %d 秒", probe.Duration, minimumSeconds)
	}
	if sourceDuration > 0 && sourceDuration <= 6*60*60 && probe.Duration+5 < sourceDuration*minCompletedDurationRatio {
		return fmt.Errorf(
			"视频疑似下载截断: 成片 %.2f 秒，源声明 %.2f 秒，低于 %.0f%%",
			probe.Duration, sourceDuration, minCompletedDurationRatio*100)
	}
	return nil
}

func (d *M3U8Downloader) minDurationSeconds() int {
	if d.cfg == nil {
		return defaultMinStreamDurationSeconds
	}
	if d.cfg.StreamMinDurationSeconds < 0 {
		return 0
	}
	return d.cfg.StreamMinDurationSeconds
}

// Cancel 取消下载
func (d *M3U8Downloader) Cancel(taskID string) bool {
	if val, ok := d.activeProcs.Load(taskID); ok {
		cmd := val.(*exec.Cmd)
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
		}
		return true
	}
	return false
}

func (d *M3U8Downloader) buildFFmpegCmd(videoURL, outputPath, videoType, referer string) *exec.Cmd {
	args := []string{"-y"}

	// HLS 特殊处理：ffmpeg 8+ 严格检查分片扩展名，而很多流媒体源的分片
	// 是 query string（没扩展名）或自定义扩展名，需要关闭这些检查
	if videoType == "m3u8" {
		args = append(args,
			"-extension_picky", "0",
			"-allowed_extensions", "ALL",
			"-allowed_segment_extensions", "ALL",
		)
	}

	// Headers 需要在 -i 之前
	if referer != "" {
		args = append(args, "-headers", fmt.Sprintf("Referer: %s\r\n", referer))
	}

	args = append(args, "-i", videoURL)

	// 始终使用流复制避免重编码
	args = append(args, "-c", "copy")
	if videoType == "m3u8" {
		args = append(args, "-bsf:a", "aac_adtstoasc")
	}

	args = append(args, "-progress", "pipe:2", outputPath)

	return exec.Command(d.cfg.FFMPEGPath, args...)
}

func (d *M3U8Downloader) parseFFmpegProgress(line string, totalDuration float64) (float64, int64, bool) {
	// 尝试 out_time_ms 格式
	if matches := outTimeMsRe.FindStringSubmatch(line); len(matches) > 1 {
		if ms, err := strconv.ParseInt(matches[1], 10, 64); err == nil && totalDuration > 0 {
			pct := float64(ms) / 1e6 / totalDuration * 100
			if pct > 100 {
				pct = 100
			}
			return pct, 0, true
		}
	}

	// 尝试 time= 格式
	if matches := timeRe.FindStringSubmatch(line); len(matches) > 1 {
		hours, _ := strconv.ParseFloat(matches[1], 64)
		minutes, _ := strconv.ParseFloat(matches[2], 64)
		seconds, _ := strconv.ParseFloat(matches[3], 64)
		currentTime := hours*3600 + minutes*60 + seconds

		if totalDuration > 0 {
			pct := currentTime / totalDuration * 100
			if pct > 100 {
				pct = 100
			}
			return pct, 0, true
		}
	}

	// 尝试 size 格式
	if matches := sizeRe.FindStringSubmatch(line); len(matches) > 1 {
		if bytes, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			// size/total_size 行只包含字节数，不包含可计算的播放进度。
			// 使用负值表示“不要覆盖当前进度”，避免每次字节更新都把进度重置为 0。
			return -1, bytes, true
		}
	}

	return 0, 0, false
}

// getM3U8Duration 获取视频总时长（m3u8 / mp4 都支持）
func (d *M3U8Downloader) getM3U8Duration(ctx context.Context, videoURL, referer string) float64 {
	// 使用 ffprobe 获取时长（对 m3u8 / mp4 都有效）
	args := []string{"-v", "quiet", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1"}
	if referer != "" {
		args = append(args, "-headers", fmt.Sprintf("Referer: %s\r\n", referer))
	}
	args = append(args, videoURL)

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0
	}

	return duration
}
