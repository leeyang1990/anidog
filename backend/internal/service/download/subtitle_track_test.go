package download

import (
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediacheck"
	"reflect"
	"testing"
)

func TestMP4TrackNamesAgreeBetweenAdmissionAndSavedEvidence(t *testing.T) {
	media := mediacheck.Media{Streams: []mediacheck.Stream{
		{CodecType: "video", Width: 1920, Height: 1080},
		{CodecType: "audio", Tags: map[string]string{"language": "jpn"}},
		{CodecType: "subtitle", CodecName: "mov_text", Tags: map[string]string{"language": "chi", "name": "中文（简体）", "handler_name": "SubtitleHandler"}},
		{CodecType: "subtitle", CodecName: "mov_text", Tags: map[string]string{"language": "zho", "name": "中文（繁體）", "handler_name": "SubtitleHandler"}},
	}}
	assessment := AssessMediaPreferences(media, "", "", "1080p", []string{"simplified"})
	if assessment.Status != model.QualityMeetsTarget {
		t.Fatal(assessment)
	}
	if tracks := subtitleTrackLanguages(media); !reflect.DeepEqual(tracks, []string{"simplified", "traditional"}) {
		t.Fatal(tracks)
	}
	// The same labels on an audio/data stream cannot become subtitle proof.
	media.Streams[2].CodecType = "data"
	assessment = AssessMediaPreferences(media, "", "", "1080p", []string{"simplified"})
	if assessment.Status == model.QualityMeetsTarget {
		t.Fatal("non-subtitle metadata granted admission")
	}
	media.Streams = media.Streams[:2]
	media.Streams = append(media.Streams, mediacheck.Stream{CodecType: "subtitle", Tags: map[string]string{"language": "chi", "name": "中文"}})
	if got := AssessMediaPreferences(media, "", "", "1080p", []string{"simplified"}); got.Status != model.QualityUnknown {
		t.Fatal("ambiguous Chinese became simplified", got)
	}
}
