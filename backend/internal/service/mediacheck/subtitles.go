package mediacheck

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Only unambiguous script-specific characters count; common Han characters do
// not establish simplified/traditional. Mixed or sparse evidence stays unknown.
const simplifiedMarkers = "这来时为们体对发后当无与万从应还战爱见开进实动长场问样现点头关东书车么龙话说听学国会过边让给将变经种气门间选亲脸声觉总数广远连动带写认条欢观转达别岛师岁难复风画广业欢游读记钱错试语诗轻压层级线红绿蓝岛"
const traditionalMarkers = "這來時為們體對發後當無與萬從應還戰愛見開進實動長場問樣現點頭關東書車麼龍話說聽學國會過邊讓給將變經種氣門間選親臉聲覺總數廣遠連動帶寫認條歡觀轉達別島師歲難複風畫廣業歡遊讀記錢錯試語詩輕壓層級線紅綠藍島遼闊"

type SubtitleEvidence struct {
	Language   string `json:"language"`
	Frames     int    `json:"frames"`
	Characters int    `json:"characters"`
	Reason     string `json:"reason"`
}

type scriptVotes struct {
	simplified, traditional                  map[rune]bool
	simpleFrames, traditionalFrames, chinese int
}

func (v *scriptVotes) addTSV(raw string) {
	if v.simplified == nil {
		v.simplified = map[rune]bool{}
		v.traditional = map[rune]bool{}
	}
	simple, traditional := false, false
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.SplitN(line, "\t", 12)
		if len(fields) != 12 || fields[0] != "5" {
			continue
		}
		confidence, err := strconv.ParseFloat(fields[10], 64)
		if err != nil || confidence < 80 {
			continue
		}
		for _, r := range fields[11] {
			if unicode.Is(unicode.Han, r) {
				v.chinese++
			}
			if strings.ContainsRune(simplifiedMarkers, r) {
				v.simplified[r] = true
				simple = true
			}
			if strings.ContainsRune(traditionalMarkers, r) {
				v.traditional[r] = true
				traditional = true
			}
		}
	}
	if simple {
		v.simpleFrames++
	}
	if traditional {
		v.traditionalFrames++
	}
}

func (v *scriptVotes) result() SubtitleEvidence {
	e := SubtitleEvidence{Characters: v.chinese, Reason: "抽样字幕证据不足，不能确认简繁体"}
	if v.chinese < 20 {
		return e
	}
	// Multiple frames and distinct discriminating glyphs prevent a single OCR
	// substitution, an opening credit or a watermark from proving compliance.
	if v.simpleFrames >= 3 && len(v.simplified) >= 6 && len(v.traditional) == 0 {
		e.Language, e.Frames, e.Reason = "simplified", v.simpleFrames, "多个抽样画面检测到简体硬字幕"
	}
	if v.traditionalFrames >= 2 && len(v.traditional) >= 3 && len(v.simplified) == 0 {
		e.Language, e.Frames, e.Reason = "traditional", v.traditionalFrames, "多个抽样画面检测到繁体硬字幕"
	}
	return e
}

// DetectHardSubtitles reads a few subtitle-sized image regions. It never
// downloads a complete episode and it never substitutes a requested script
// when OCR is missing or uncertain. Tools are optional for desktop installs.
func DetectHardSubtitles(ctx context.Context, input string, inputArgs []string, duration float64) SubtitleEvidence {
	if _, err := exec.LookPath("tesseract"); err != nil {
		return SubtitleEvidence{Reason: "未安装字幕识别工具，无法确认硬字幕"}
	}
	ctx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()
	votes := scriptVotes{}
	for i, at := range []float64{120, 180, 300, 600} {
		if duration > 0 && duration < 750 {
			at = duration * []float64{0.16, 0.26, 0.42, 0.65}[i]
		}
		frameCtx, stopFrame := context.WithTimeout(ctx, 18*time.Second)
		args := []string{"-nostdin", "-v", "error", "-rw_timeout", "10000000", "-ss", fmt.Sprintf("%.3f", at)}
		args = append(args, hlsInputArgs("ffmpeg", input, "", inputArgs)...)
		args = append(args, "-i", input, "-map", "0:v:0", "-frames:v", "1", "-an", "-sn", "-vf", "crop=iw:ih*0.20:0:ih*0.80,scale=1920:-2", "-threads", "1", "-f", "image2pipe", "-vcodec", "png", "pipe:1")
		frame, err := MediaCommand(frameCtx, "ffmpeg", args...).Output()
		stopFrame()
		if err != nil {
			continue
		}
		ocrCtx, stopOCR := context.WithTimeout(ctx, 8*time.Second)
		ocr := exec.CommandContext(ocrCtx, "tesseract", "stdin", "stdout", "-l", "chi_tra+chi_sim", "--psm", "6", "-c", "tessedit_create_tsv=1")
		ocr.Stdin = bytes.NewReader(frame)
		out, err := ocr.Output()
		stopOCR()
		if err == nil {
			votes.addTSV(string(out))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return votes.result()
}
