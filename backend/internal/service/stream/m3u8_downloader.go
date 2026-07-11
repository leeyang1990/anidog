package stream

import (
	"bufio"
	"context"
	"fmt"
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

	// 下载成功后检测文件是否真实视频（反盗链返回 PNG/JPG 假流的防护）
	if !d.isValidVideo(outputPath) {
		_ = os.Remove(outputPath)
		return "", fmt.Errorf("下载的文件不是有效视频（可能源返回了伪装流，尝试切换到其他源）")
	}

	zap.L().Info("ffmpeg 下载完成", zap.String("output", outputPath))
	return outputPath, nil
}

// isValidVideo 用 ffprobe 检查输出文件是否包含真实视频流
// 防止反盗链源返回 PNG/JPG 序列等伪装流
func (d *M3U8Downloader) isValidVideo(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() < 100*1024 {
		return false // 文件太小或不存在
	}

	cmd := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path)
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	codec := strings.TrimSpace(strings.ToLower(string(out)))
	// 真实视频编码：h264, h265/hevc, av1, vp9 等
	// 伪装流：png, mjpeg, jpeg
	if codec == "" || codec == "png" || codec == "mjpeg" || codec == "jpeg" {
		zap.L().Warn("检测到伪装视频流",
			zap.String("path", path),
			zap.String("codec", codec))
		return false
	}
	return true
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
