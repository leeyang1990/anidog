package mediacheck

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestConflictingOCRReadsContributeOnlyAgreedGlyphs(t *testing.T) {
	agreed := combineCaptions("我们已經理解原因", "我们已经完全理解原因")
	if strings.ContainsAny(agreed, "经經") {
		t.Fatal("an uncertain glyph was counted as script evidence", agreed)
	}
	var votes scriptVotes
	for _, caption := range []string{"这是我们的世界这个国家还有故事", "我们听见这里的对话正在发展", "这个学校还有更多事情让我们认识"} {
		votes.addCaption(caption)
	}
	votes.addCaption(agreed)
	votes.addCaption("那用倒是別不住了")
	if votes.result().Language != "simplified" {
		t.Fatal("one disputed OCR glyph poisoned independently verified simplified captions", votes.result())
	}
	// A real traditional caption that both reads agree on still blocks a
	// mixed result; preprocessing cannot rewrite it as simplified evidence.
	traditional := "這個國家的故事還沒有結束我們會再見面"
	votes.addCaption(combineCaptions(traditional, traditional))
	if votes.result().Language != "" {
		t.Fatal("confirmed mixed-script evidence was ignored", votes.result())
	}
	if got := combineCaptions("這個國家會再見面", "这个国家会再见面"); hasScriptMarker(got, simplifiedMarkers) || hasScriptMarker(got, traditionalMarkers) {
		t.Fatal("conflicting transformations invented script evidence", got)
	}
}

func TestInvalidOCRConfidenceCannotProveSubtitleLanguage(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "101", "-1"} {
		raw := strings.Replace(subtitleTSV("这是我们的世界这个国家学校还有许多故事对话发展", 96), "\t96\t", "\t"+value+"\t", 1)
		if subtitleText(raw) != "" {
			t.Fatal("invalid OCR confidence admitted", value)
		}
	}
}

func TestSubtitleMaskPreservesWhiteCaptionAgainstColoredBackground(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 3, 1))
	picture.Set(0, 0, color.RGBA{255, 255, 255, 255})
	picture.Set(1, 0, color.RGBA{245, 208, 139, 255})
	picture.Set(2, 0, color.RGBA{0, 0, 0, 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	mask, err := subtitleMask(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(mask))
	if err != nil {
		t.Fatal(err)
	}
	for x, want := range []uint32{0, 65535, 65535} {
		r, _, _, _ := decoded.At(x, 0).RGBA()
		if r != want {
			t.Fatalf("caption/background polarity lost at pixel %d", x)
		}
	}
}

func TestSubtitleExecutionFailureIsNotLanguageRejection(t *testing.T) {
	for _, stage := range []string{"frame", "ocr", "empty", "simplified", "traditional"} {
		t.Run(stage, func(t *testing.T) {
			recognized := 0
			e := detectHardSubtitles(context.Background(), 1500, func(ctx context.Context, at float64) ([]byte, error) {
				if deadline, ok := ctx.Deadline(); !ok || deadline.Sub(time.Now()) < 40*time.Second {
					t.Fatal("network seek budget too short")
				}
				if stage == "frame" {
					return nil, context.DeadlineExceeded
				}
				return []byte("frame"), nil
			}, func(context.Context, []byte) (string, error) {
				if stage == "ocr" {
					return "", errors.New("language data unavailable")
				}
				if stage == "simplified" {
					recognized++
					return fmt.Sprintf("这是我们的世界这个国家学校还有许多故事对话发展%c", rune(0x4e00+recognized)), nil
				}
				if stage == "traditional" {
					recognized++
					return fmt.Sprintf("這個國家的故事還沒有結束我們會再見面%c", rune(0x4e00+recognized)), nil
				}
				return "", nil
			})
			if e.Retryable != (stage == "frame" || stage == "ocr") {
				t.Fatalf("%+v", e)
			}
			if stage == "empty" && (e.Language != "" || e.Recognized < 3) {
				t.Fatal(e)
			}
			if stage == "simplified" && e.Language != "simplified" {
				t.Fatal(e)
			}
			if stage == "traditional" && e.Language != "traditional" {
				t.Fatal(e)
			}
		})
	}
}

func subtitleTSV(text string, confidence int) string {
	return fmt.Sprintf("5\t1\t1\t1\t1\t1\t0\t0\t100\t30\t%d\t%s\n", confidence, text)
}

func TestSubtitleScriptRequiresDistinctHighConfidenceFrames(t *testing.T) {
	var v scriptVotes
	v.addTSV(subtitleTSV("等你出了社會了解這世界有多遼闊", 95))
	if v.result().Language != "" {
		t.Fatal("one frame must not establish subtitle language")
	}
	v.addTSV(subtitleTSV("這個國家的故事還沒有結束我們會再見面", 96))
	if v.result().Language != "traditional" {
		t.Fatal(v.result())
	}
	v.addTSV(subtitleTSV("这是一个新的世界", 96))
	if v.result().Language != "" {
		t.Fatal("conflicting script evidence must stay unknown")
	}
	var simple scriptVotes
	for i := 0; i < 3; i++ {
		simple.addTSV(subtitleTSV(fmt.Sprintf("这是我们的世界这个国家学校还有许多故事对话发展%c", rune(0x4e00+i)), 96))
	}
	if simple.result().Language != "simplified" {
		t.Fatal(simple.result())
	}
	var repeated scriptVotes
	for i := 0; i < 10; i++ {
		repeated.addTSV(subtitleTSV("这是我们的世界这个国家学校还有许多故事对话发展", 96))
	}
	if repeated.result().Language != "" {
		t.Fatal("one repeated caption was counted as independent subtitle evidence")
	}
	var low scriptVotes
	for i := 0; i < 4; i++ {
		low.addTSV(subtitleTSV("这是我们的世界这个国家学校还有许多故事对话发展", 40))
	}
	if low.result().Language != "" {
		t.Fatal("low confidence OCR cannot establish compliance")
	}
}

func TestSubtitleVotesKeepMainCaptionSeparateFromSmallerJapaneseRow(t *testing.T) {
	var votes scriptVotes
	for i, caption := range []string{"这是我们的世界这个国家学校还有许多故事对话发展", "我们还会来到这个世界听见他们说的话", "这里的故事让我们看到对方的真实想法"} {
		main := fmt.Sprintf("5\t1\t1\t1\t1\t1\t500\t90\t800\t50\t96\t%s\n", caption)
		translation := fmt.Sprintf("5\t1\t1\t1\t2\t1\t500\t160\t800\t30\t96\t%s\n", []string{"世界國見體後為會", "見國體時後會學", "世界體後時見會"}[i])
		votes.addTSV(main + translation)
	}
	if votes.result().Language != "simplified" {
		t.Fatal("Japanese kanji contaminated Chinese script evidence", votes.result())
	}
	if got := subtitleText("5\t1\t1\t1\t1\t1\t1\t1\t100\t50\t96\t為\n"); got != "" {
		t.Fatal("isolated background OCR counted as a subtitle", got)
	}
}

func TestJapaneseSharedGlyphsCannotEstablishSimplifiedChinese(t *testing.T) {
	var votes scriptVotes
	for i := 0; i < 4; i++ {
		votes.addTSV(subtitleTSV(fmt.Sprintf("世界国家体社会来万当点与将数写条画没据%c", rune(0x4e00+i)), 96))
	}
	if votes.result().Language != "" {
		t.Fatal("Japanese/shared characters were labeled as simplified Chinese", votes.result())
	}
}

func TestSubtitleSamplesStayInsideShortAndMediumEpisodes(t *testing.T) {
	for _, duration := range []float64{90, 600, 780, 900, 1200, 1500} {
		count := 0
		detectHardSubtitles(context.Background(), duration, func(_ context.Context, at float64) ([]byte, error) {
			if at <= 0 || at >= duration {
				t.Fatalf("sample %.1f outside %.1fs episode", at, duration)
			}
			count++
			return []byte("frame"), nil
		}, func(context.Context, []byte) (string, error) { return "", nil })
		if count < 3 {
			t.Fatal("not enough independent samples", duration, count)
		}
	}
}
