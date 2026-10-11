package titleparse

import (
	"regexp"
	"strconv"
	"strings"
)

var sequelNumber = regexp.MustCompile(`(?:\s|：|:)\s*([2-9])\s*$`)
var arcName = regexp.MustCompile(`[\p{Han}]{2,8}[篇編谭譚]`)
var arcNormalizer = strings.NewReplacer("奪", "夺", "還", "还", "喪", "丧", "編", "篇", "譚", "谭", "禍", "祸", "進", "进", "戰", "战")

// InstallmentNumber reads a catalog title, independent of library folder season.
func InstallmentNumber(title string) int { return installmentSeason(title, true) }

// InstallmentCompatible checks identity before any popularity/preference score.
// A missing sequel/arc marker is not evidence that the release is the sequel.
// Episode numbers in release brackets must never become season numbers.
func InstallmentCompatible(target, candidate string) bool {
	targetSeason := installmentSeason(target, true)
	candidateSeason := installmentSeason(candidate, false)
	if targetSeason > 1 && targetSeason != candidateSeason {
		return false
	}
	if targetSeason <= 1 && candidateSeason > 1 {
		return false
	}
	targetArcs := arcName.FindAllString(reSeasonChinese.ReplaceAllString(arcNormalizer.Replace(target), ""), -1)
	for _, want := range targetArcs {
		if !strings.Contains(arcNormalizer.Replace(candidate), want) {
			return false
		}
	}
	return true
}

func installmentSeason(title string, target bool) int {
	if n := detectSeasonNumber(title); n != nil {
		return *n
	}
	// Plain catalog titles can end with a sequel number. For releases inspect
	// only the parsed series name, never their trailing episode/quality tokens.
	name := title
	if !target && (strings.ContainsAny(title, "[]【】") || reEpisodeDash.MatchString(title)) {
		name = Parse(title).AnimeName
	}
	if m := sequelNumber.FindStringSubmatch(name); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 1
}
