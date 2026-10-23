package mediacheck

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Only unambiguous script-specific characters count; common Han characters do
// not establish simplified/traditional. Mixed or sparse evidence stays unknown.
const simplifiedMarkers = "历谓灵阶证据处谁离够饭鸡鱼热锅饱烧饲梦愿报结紧术练预务个没吗贝两弃这来时为们体对发后当无与万从应还战爱见开进实动长场问样现点头关东书车么龙话说听学国会过边让给将变经种气门间选亲脸声觉总数广远连动带写认条欢观转达岛师岁难复风画广业欢游读记钱错试语诗轻压层级线红绿蓝岛"
const traditionalMarkers = "歷曆謂靈階證據處誰離夠飯雞魚熱鍋飽燒飼夢願報結緊術練預務個沒嗎貝兩棄這來時為們體對發後當無與萬從應還戰愛見開進實動長場問樣現點頭關東書車麼龍話說聽學國會過邊讓給將變經種氣門間選親臉聲覺總數廣遠連動帶寫認條歡觀轉達島師歲難複風畫廣業歡遊讀記錢錯試語詩輕壓層級線紅綠藍島遼闊"

// 别/別 differ by a tiny stroke and were misread at high confidence in real
// subtitle crops. Neither is reliable script evidence; require other glyphs.
// Shared shinjitai characters alone cannot establish Chinese subtitles.
const sharedJapaneseMarkers = "来体万当与点学国会将声数写条画没据"

type SubtitleEvidence struct {
	Language    string `json:"language"`
	Frames      int    `json:"frames"`
	Characters  int    `json:"characters"`
	Reason      string `json:"reason"`
	Attempted   int    `json:"attempted"`
	Extracted   int    `json:"extracted"`
	Recognized  int    `json:"recognized"`
	FrameErrors int    `json:"frame_errors"`
	OCRErrors   int    `json:"ocr_errors"`
	Retryable   bool   `json:"retryable"`
}

type scriptVotes struct {
	simplified, traditional                  map[rune]bool
	seen                                     map[string]bool
	simpleFrames, traditionalFrames, chinese int
}

// subtitleText rejects isolated background OCR and the smaller translation row
// of bilingual subtitles. Classify one coherent caption band, rather than
// mixing Chinese with Japanese kanji recognized by the Chinese OCR model.
func subtitleText(raw string) string {
	type line struct {
		text    strings.Builder
		heights []int
	}
	lines := map[string]*line{}
	order := []string{}
	for _, row := range strings.Split(raw, "\n") {
		fields := strings.SplitN(row, "\t", 12)
		if len(fields) != 12 || fields[0] != "5" {
			continue
		}
		confidence, err := strconv.ParseFloat(fields[10], 64)
		height, heightErr := strconv.Atoi(fields[9])
		if err != nil || heightErr != nil || !(confidence >= 80 && confidence <= 100) || height < 20 {
			continue
		}
		key := strings.Join(fields[1:5], ":")
		caption := lines[key]
		if caption == nil {
			caption = &line{}
			lines[key] = caption
			order = append(order, key)
		}
		for _, r := range fields[11] {
			if unicode.Is(unicode.Han, r) {
				caption.text.WriteRune(r)
				caption.heights = append(caption.heights, height)
			}
		}
	}
	largest := 0
	for _, caption := range lines {
		if len(caption.heights) < 4 {
			continue
		}
		sort.Ints(caption.heights)
		largest = max(largest, caption.heights[len(caption.heights)/2])
	}
	var text strings.Builder
	for _, key := range order {
		caption := lines[key]
		if len(caption.heights) >= 4 && caption.heights[len(caption.heights)/2]*5 >= largest*4 {
			text.WriteString(caption.text.String())
		}
	}
	return text.String()
}

func (v *scriptVotes) addTSV(raw string) { v.addCaption(subtitleText(raw)) }

func (v *scriptVotes) addCaption(text string) {
	if text == "" {
		return
	}
	if v.seen == nil {
		v.seen = make(map[string]bool)
		v.simplified = map[rune]bool{}
		v.traditional = map[rune]bool{}
	}
	if v.seen[text] {
		return
	}
	v.seen[text] = true
	simple, traditional := false, false
	for _, r := range text {
		v.chinese++
		if strings.ContainsRune(simplifiedMarkers, r) {
			v.simplified[r] = true
			if !strings.ContainsRune(sharedJapaneseMarkers, r) {
				simple = true
			}
		}
		if strings.ContainsRune(traditionalMarkers, r) {
			v.traditional[r] = true
			traditional = true
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
	specific := 0
	for r := range v.simplified {
		if !strings.ContainsRune(sharedJapaneseMarkers, r) {
			specific++
		}
	}
	if v.simpleFrames >= 3 && specific >= 2 && len(v.simplified) >= 6 && len(v.traditional) == 0 {
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
		return SubtitleEvidence{Reason: "未安装字幕识别工具，无法确认硬字幕", Retryable: true}
	}
	return detectHardSubtitles(ctx, duration, func(ctx context.Context, at float64) ([]byte, error) {
		args := []string{"-nostdin", "-v", "error", "-rw_timeout", "15000000", "-ss", fmt.Sprintf("%.3f", at)}
		args = append(args, hlsInputArgs("ffmpeg", input, "", inputArgs)...)
		args = append(args, "-i", input, "-map", "0:v:0", "-frames:v", "1", "-an", "-sn", "-vf", "crop=iw:ih*0.20:0:ih*0.80,scale=1920:-2", "-threads", "1", "-f", "image2pipe", "-vcodec", "png", "pipe:1")
		return MediaCommand(ctx, "ffmpeg", args...).Output()
	}, recognizeSubtitleFrame)
}

// White outlined captions on busy anime backgrounds defeat block OCR even
// at full resolution. A second high-contrast read may recover the caption;
// it is still judged by the same confidence and script evidence thresholds.
func recognizeSubtitleFrame(ctx context.Context, frame []byte) (string, error) {
	return recognizeCaption(ctx, frame, func(ctx context.Context, input []byte) (string, error) {
		ocr := exec.CommandContext(ctx, "tesseract", "stdin", "stdout", "-l", "chi_tra+chi_sim", "--psm", "6", "-c", "tessedit_create_tsv=1")
		ocr.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1", "OMP_NUM_THREADS=1")
		ocr.Stdin = bytes.NewReader(input)
		raw, err := ocr.Output()
		if err != nil {
			return "", err
		}
		return subtitleText(string(raw)), nil
	})
}

func recognizeCaption(ctx context.Context, frame []byte, read func(context.Context, []byte) (string, error)) (string, error) {
	// A complex background must not consume both OCR passes' entire budget.
	// Each pass gets its own 20-second budget; either valid
	// high-confidence read can still provide evidence when the other times out.
	bounded := func(input []byte) (string, error) {
		passCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		return read(passCtx, input)
	}
	original, originalErr := bounded(frame)
	originalCount := len([]rune(original))
	if originalErr == nil && originalCount >= 8 && !hasScriptMarker(original, traditionalMarkers) {
		return original, nil
	}
	mask, err := subtitleMask(frame)
	if err != nil {
		return original, originalErr
	}
	masked, err := bounded(mask)
	if err != nil {
		if originalErr == nil && original != "" {
			return original, nil
		}
		return "", err
	}
	if originalErr != nil {
		if len([]rune(masked)) < 4 {
			return "", originalErr
		}
		return masked, nil
	}
	return combineCaptions(original, masked), nil
}

func hasScriptMarker(text, markers string) bool {
	for _, r := range text {
		if strings.ContainsRune(markers, r) {
			return true
		}
	}
	return false
}

func combineCaptions(original, masked string) string {
	// For a script conflict, only agreed characters are evidence. A single
	// uncertain glyph must not poison an otherwise confirmed simplified episode;
	// a traditional caption must not turn simplified just because of filtering.
	conflict := (hasScriptMarker(original, traditionalMarkers) && hasScriptMarker(masked, simplifiedMarkers)) || (hasScriptMarker(masked, traditionalMarkers) && hasScriptMarker(original, simplifiedMarkers))
	if conflict {
		return commonCaption(original, masked)
	}
	if len([]rune(masked)) > len([]rune(original)) {
		return masked
	}
	return original
}

// Longest common subsequence preserves reading order through OCR insertions
// and omissions. It never invents glyphs or assigns synthetic confidence.
func commonCaption(first, second string) string {
	a, b := []rune(first), []rune(second)
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = 1 + table[i+1][j+1]
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	var agreed strings.Builder
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if a[i] == b[j] {
			agreed.WriteRune(a[i])
			i++
			j++
		} else if table[i+1][j] >= table[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return agreed.String()
}

func subtitleMask(frame []byte) ([]byte, error) {
	picture, err := png.Decode(bytes.NewReader(frame))
	if err != nil {
		return nil, err
	}
	bounds := picture.Bounds()
	mask := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			value := color.GrayModel.Convert(picture.At(x, y)).(color.Gray).Y
			shade := uint8(255)
			if value >= 240 {
				shade = 0
			}
			mask.SetGray(x, y, color.Gray{Y: shade})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, mask); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// A network seek can legitimately exceed 18s. Keep an overall bound, allow
// 45s for each seek and report execution failure separately from OCR evidence.
func detectHardSubtitles(ctx context.Context, duration float64, extract func(context.Context, float64) ([]byte, error), recognize func(context.Context, []byte) (string, error)) SubtitleEvidence {
	ctx, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()
	votes := scriptVotes{}
	evidence := SubtitleEvidence{}
	// Some exact timestamps land in a gap, credits, or a line containing only
	// shared Han characters. Sample additional dialogue without lowering the
	// confidence, script-specific glyph, or independent-frame requirements.
	for i, at := range []float64{300, 615, 915, 1020, 420, 720, 120, 135, 315, 600} {
		if ctx.Err() != nil {
			break
		}
		if duration > 0 && (duration < 750 || at >= duration*0.9) {
			at = duration * []float64{0.21, 0.42, 0.63, 0.70, 0.29, 0.50, 0.08, 0.09, 0.22, 0.41}[i]
		}
		evidence.Attempted++
		frameCtx, stopFrame := context.WithTimeout(ctx, 45*time.Second)
		frame, err := extract(frameCtx, at)
		stopFrame()
		if err != nil || len(frame) == 0 {
			evidence.FrameErrors++
			continue
		}
		evidence.Extracted++
		ocrCtx, stopOCR := context.WithTimeout(ctx, 40*time.Second)
		out, err := recognize(ocrCtx, frame)
		stopOCR()
		if err != nil {
			evidence.OCRErrors++
			continue
		}
		evidence.Recognized++
		votes.addCaption(out)
		if votes.result().Language != "" {
			break
		}
	}
	result := votes.result()
	evidence.Language, evidence.Frames, evidence.Characters, evidence.Reason = result.Language, result.Frames, result.Characters, result.Reason
	if evidence.Language == "" && (evidence.FrameErrors > 0 || evidence.OCRErrors > 0 || ctx.Err() != nil) {
		evidence.Retryable = true
		evidence.Reason = fmt.Sprintf("字幕检测执行失败：抽帧 %d/%d 成功（%d 失败），识别 %d 成功（%d 失败），等待重试", evidence.Extracted, evidence.Attempted, evidence.FrameErrors, evidence.Recognized, evidence.OCRErrors)
	}
	return evidence
}
