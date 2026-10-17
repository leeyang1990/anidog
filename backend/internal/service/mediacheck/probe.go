// Package mediacheck inspects the actual downloaded file before it can become
// library media. Transfer completion alone is not a media-quality verdict.
package mediacheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Tool availability and deadlines are retryable, not evidence of bad media.
type toolError struct{ error }

func IsToolError(err error) bool {
	var target toolError
	return errors.As(err, &target)
}

func commandError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return toolError{ctx.Err()}
	}
	var unavailable *exec.Error
	if errors.As(err, &unavailable) || errors.Is(err, os.ErrPermission) {
		return toolError{err}
	}
	return err
}

type Stream struct {
	CodecType   string            `json:"codec_type"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Tags        map[string]string `json:"tags"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}
type Media struct {
	Streams []Stream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
		Size     string `json:"size"`
	} `json:"format"`
}

func Probe(ctx context.Context, path string) (Media, error) {
	return ProbeInput(ctx, path, nil)
}

// InputArgs contains only transport headers, placed before the input URL.
func InputArgs(referer, userAgent string) []string {
	args := []string{}
	if referer != "" {
		args = append(args, "-referer", referer)
	}
	if userAgent != "" {
		args = append(args, "-user_agent", userAgent)
	}
	return args
}

func ProbeInput(ctx context.Context, path string, inputArgs []string) (Media, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := []string{"-v", "error", "-rw_timeout", "15000000", "-analyzeduration", "3000000", "-probesize", "3000000", "-show_entries", "stream=codec_type,width,height:stream_tags=language,title:stream_disposition=attached_pic:format=duration,bit_rate,size", "-of", "json"}
	args = append(args, inputArgs...)
	args = append(args, path)
	out, err := MediaCommand(ctx, "ffprobe", args...).Output()
	var m Media
	if err != nil {
		return m, fmt.Errorf("无法读取成片: %w", commandError(ctx, err))
	}
	if err := json.Unmarshal(out, &m); err != nil {
		return m, err
	}
	if m.Duration() <= 0 {
		return m, fmt.Errorf("成片时长无效")
	}
	if w, h := m.Resolution(); w == 0 || h == 0 {
		return m, fmt.Errorf("成片没有视频轨道")
	}
	return m, nil
}

func (m Media) Duration() float64 {
	n, _ := strconv.ParseFloat(m.Format.Duration, 64)
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	return n
}
func (m Media) Resolution() (int, int) {
	w, h := 0, 0
	for _, s := range m.Streams {
		if s.CodecType == "video" && s.Disposition.AttachedPic == 0 && s.Width*s.Height > w*h {
			w, h = s.Width, s.Height
		}
	}
	return w, h
}

// CheckTail performs a bounded real decode, rather than trusting an MP4 index
// which can survive an incomplete last packet. It is not a full-file checksum.
func CheckTail(ctx context.Context, path string, duration float64) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	at := math.Max(0, duration-12)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-xerror", "-threads", "1", "-ss", fmt.Sprintf("%.3f", at), "-noaccurate_seek", "-skip_frame", "nokey", "-i", path, "-map", "0:v:0", "-t", "30", "-an", "-sn", "-f", "null", "-", "-progress", "pipe:1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if classified := commandError(ctx, err); IsToolError(classified) {
			return classified
		}
	}
	decoded := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "frame=") {
			n, _ := strconv.Atoi(strings.TrimPrefix(line, "frame="))
			if n > 0 {
				decoded = true
			}
		}
	}
	if err != nil || !decoded || strings.Contains(string(out), "Invalid data") || strings.Contains(string(out), "Invalid NAL") {
		if len(out) > 1200 {
			out = out[:1200]
		}
		return fmt.Errorf("成片尾部无法正常解码: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
