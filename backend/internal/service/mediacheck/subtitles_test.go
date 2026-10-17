package mediacheck

import (
	"fmt"
	"testing"
)

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
		simple.addTSV(subtitleTSV("这是我们的世界这个国家学校还有许多故事对话发展", 96))
	}
	if simple.result().Language != "simplified" {
		t.Fatal(simple.result())
	}
	var low scriptVotes
	for i := 0; i < 4; i++ {
		low.addTSV(subtitleTSV("这是我们的世界这个国家学校还有许多故事对话发展", 40))
	}
	if low.result().Language != "" {
		t.Fatal("low confidence OCR cannot establish compliance")
	}
}
