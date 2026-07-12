package titleparse

import "testing"

func TestParseSeasonNumber(t *testing.T) {
	tests := []struct {
		title string
		want  int
	}{
		{"[DBD-Raws][无职转生 第二季][01-25]", 2},
		{"[Feibanyama] Mushoku Tensei S03E02", 3},
		{"Mushoku Tensei III： Isekai Ittara Honki Dasu [01]", 3},
		{"Anime 2nd Season - 03", 2},
	}
	for _, tt := range tests {
		parsed := Parse(tt.title)
		if parsed.SeasonNum == nil || *parsed.SeasonNum != tt.want {
			t.Errorf("Parse(%q).SeasonNum = %v, want %d", tt.title, parsed.SeasonNum, tt.want)
		}
	}
}
