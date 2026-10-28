package mediacheck

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func outlinedFixture(t *testing.T, name string) []byte {
	t.Helper()
	frame, err := os.ReadFile("testdata/outlined-" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestCaptionRegionReadsPrecedeBackgroundAndPreserveDisagreement(t *testing.T) {
	frame := outlinedFixture(t, "simplified")
	for _, conflict := range []bool{false, true} {
		calls := 0
		got, err := recognizeSubtitleFrameWith(context.Background(), frame, func(ctx context.Context, _ []byte, lang, psm string, budget time.Duration) (string, error) {
			calls++
			if psm != "7" || budget > 6*time.Second {
				t.Fatal("background OCR starved line reads", psm, budget)
			}
			if d, ok := ctx.Deadline(); !ok || time.Until(d) > 40*time.Second {
				t.Fatal("frame budget missing")
			}
			if calls == 1 && lang != "chi_tra+chi_sim" || calls == 2 && lang != "chi_sim+chi_tra" {
				t.Fatal("independent script priorities missing", lang)
			}
			if conflict && calls == 2 {
				return "如果你還有其他要求不妨說說看", nil
			}
			return "如果你还有其他要求不妨说说看", nil
		})
		if err != nil || calls != 2 || got == "" {
			t.Fatal(got, err, calls)
		}
		if conflict && strings.ContainsAny(got, "还還说說") {
			t.Fatal("normalization dispute became evidence", got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := recognizeSubtitleFrameWith(ctx, frame, func(context.Context, []byte, string, string, time.Duration) (string, error) {
		t.Fatal("OCR launched after cancellation")
		return "", nil
	})
	if err != context.Canceled {
		t.Fatal("cancellation lost", err)
	}
}

func TestCaptionRegionsExcludeSmallerTranslationRow(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 800, 160))
	draw.Draw(picture, picture.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for _, row := range []struct{ y, h int }{{20, 40}, {100, 24}} {
		for x := 100; x < 600; x += 60 {
			draw.Draw(picture, image.Rect(x, row.y, x+24, row.y+row.h), image.NewUniform(color.Black), image.Point{}, draw.Src)
		}
	}
	var input bytes.Buffer
	png.Encode(&input, picture)
	regions := captionRegions(input.Bytes())
	if len(regions) != 1 {
		t.Fatal("translation became an independent Chinese caption", len(regions))
	}
	im, err := png.Decode(bytes.NewReader(regions[0]))
	if err != nil || im.Bounds().Dy() > 80 {
		t.Fatal("rows merged", err)
	}
}

func TestOutlinedCaptionOCRFixtures(t *testing.T) {
	if os.Getenv("ANIDOG_OCR_FIXTURE") != "1" {
		t.Skip("set ANIDOG_OCR_FIXTURE=1 with production Chinese OCR models")
	}
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, expected string }{{"simplified", "还说"}, {"traditional", "遲話該麼"}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := recognizeSubtitleFrame(context.Background(), outlinedFixture(t, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range tc.expected {
				if !strings.ContainsRune(got, r) {
					t.Fatalf("missing actual glyph %c in %s", r, got)
				}
			}
			if tc.name == "traditional" && hasScriptMarker(got, simplifiedMarkers) {
				t.Fatal("traditional normalized to simplified", got)
			}
		})
	}
}

func TestFailedSubtitleSamplesRetainStageAndBoundedDiagnostics(t *testing.T) {
	for _, stage := range []string{"frame", "ocr"} {
		e := detectHardSubtitles(context.Background(), 1500, func(context.Context, float64) ([]byte, error) {
			if stage == "frame" {
				return nil, context.DeadlineExceeded
			}
			return []byte("frame"), nil
		}, func(context.Context, []byte) (string, error) { return "", context.DeadlineExceeded })
		if !e.Retryable || len(e.Samples) != e.Attempted {
			t.Fatal("failed samples disappeared", e)
		}
		for _, s := range e.Samples {
			if s.FailureStage != stage || s.FailureReason != "timeout" || s.At <= 0 {
				t.Fatal("missing failure evidence", s)
			}
		}
	}
}

func TestUniformCaptionBandIsNoEvidenceNotToolFailure(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 1920, 216))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	var raw bytes.Buffer
	png.Encode(&raw, im)
	got, err := recognizeSubtitleFrameWith(context.Background(), raw.Bytes(), func(context.Context, []byte, string, string, time.Duration) (string, error) {
		t.Fatal("empty caption band loaded language models")
		return "", nil
	})
	if err != nil || got != "" {
		t.Fatal("uniform frame became failure or language evidence", got, err)
	}
	im.Set(20, 20, color.RGBA{1, 0, 0, 255})
	raw.Reset()
	png.Encode(&raw, im)
	if uniformSubtitleFrame(raw.Bytes()) {
		t.Fatal("low contrast detail erased by threshold")
	}
}
