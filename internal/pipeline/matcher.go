// Package pipeline applies a sequential set of transformation rules to an
// iCalendar feed: filter, dedup, rename, strip, timezone and expire.
package pipeline

import (
	"fmt"
	"regexp"
	"strings"
)

// MatchMode selects how a pattern is interpreted.
type MatchMode string

const (
	// MatchSubstring is the "DAU" mode: case-insensitive substring match.
	MatchSubstring MatchMode = "substring"
	// MatchRegex uses full Go regular-expression syntax.
	MatchRegex MatchMode = "regex"
)

// Matcher tests strings against a configured pattern.
type Matcher struct {
	mode    MatchMode
	pattern string
	re      *regexp.Regexp // regex mode only
	lower   string         // substring mode only
}

// NewMatcher compiles a matcher. In regex mode an invalid pattern is an error.
func NewMatcher(mode MatchMode, pattern string) (*Matcher, error) {
	m := &Matcher{mode: mode, pattern: pattern}
	switch mode {
	case MatchSubstring:
		m.lower = strings.ToLower(pattern)
	case MatchRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("pipeline: invalid regex %q: %w", pattern, err)
		}
		m.re = re
	default:
		return nil, fmt.Errorf("pipeline: unknown match mode %q", mode)
	}
	return m, nil
}

// MatchString reports whether s matches the pattern.
func (m *Matcher) MatchString(s string) bool {
	switch m.mode {
	case MatchSubstring:
		return strings.Contains(strings.ToLower(s), m.lower)
	case MatchRegex:
		return m.re.MatchString(s)
	default:
		return false
	}
}
