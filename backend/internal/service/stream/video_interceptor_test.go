package stream

import "testing"

func TestPlayerFailureDoesNotTreatOuterHelpTextAsFailure(t *testing.T) {
	if playerFailureText("播放失败，请刷新或稍候再试！") == "" {
		t.Fatal("explicit player error not detected")
	}
	long := ""
	for i := 0; i < 100; i++ {
		long += "正常剧集介绍"
	}
	if playerFailureText(long+"播放失败，请刷新") != "" {
		t.Fatal("help text on an outer page must not reject media")
	}
}

func TestMediaResponseNeverSelectsSegmentsOrFailedManifests(t *testing.T) {
	for _, input := range []struct{ url, mime, want string }{
		{"https://cdn.example/seg.ts?key=x", "video/mp2t", ""},
		{"https://cdn.example/seg.m4s", "video/mp4", ""},
		{"https://cdn.example/manifest?id=1", "application/vnd.apple.mpegurl", "m3u8"},
		{"https://cdn.example/index.m3u8?key=x", "application/octet-stream", "m3u8"},
		{"https://cdn.example/hls/error", "text/html", ""},
		{"https://cdn.example/show.mp4", "video/mp4", "mp4"},
	} {
		if got := mediaResponseType(input.url, input.mime); got != input.want {
			t.Fatalf("%s: %q != %q", input.url, got, input.want)
		}
	}
}
