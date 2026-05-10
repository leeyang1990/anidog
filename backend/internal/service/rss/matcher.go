package rss

import (
	"regexp"
	"strings"

	"github.com/anidog/anidog-go/internal/model"
)

// Matcher checks if an RSS entry matches a feed's rules.
type Matcher struct{}

// NewMatcher creates a new Matcher.
func NewMatcher() *Matcher { return &Matcher{} }

// Match checks an entry title against a set of rules.
// Returns true if the entry should be downloaded.
//
// Logic:
//  1. If no rules exist, match everything (no filter).
//  2. Include rules: entry must match at least one to pass.
//  3. Exclude rules: if entry matches any, it is rejected.
//  4. If there are only exclude rules, everything not excluded passes.
func (m *Matcher) Match(title string, rules []model.RSSRule) bool {
	var includes, excludes []model.RSSRule
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if r.Include {
			includes = append(includes, r)
		} else {
			excludes = append(excludes, r)
		}
	}

	// No rules at all → match everything
	if len(includes) == 0 && len(excludes) == 0 {
		return true
	}

	// Check exclude rules first — any match means rejected
	for _, r := range excludes {
		if matchRule(title, r) {
			return false
		}
	}

	// If there are include rules, must match at least one
	if len(includes) > 0 {
		for _, r := range includes {
			if matchRule(title, r) {
				return true
			}
		}
		return false
	}

	// Only exclude rules, and none matched → pass
	return true
}

// matchRule checks if a title matches a single rule.
func matchRule(title string, rule model.RSSRule) bool {
	if rule.IsRegex {
		re, err := regexp.Compile(rule.Keyword)
		if err != nil {
			return false
		}
		return re.MatchString(title)
	}
	return strings.Contains(strings.ToLower(title), strings.ToLower(rule.Keyword))
}
