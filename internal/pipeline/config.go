package pipeline

import "fmt"

// Rule type identifiers used in stored feed configuration.
const (
	RuleFilter   = "filter"
	RuleDedup    = "dedup"
	RuleRename   = "rename"
	RuleStrip    = "strip"
	RuleTimezone = "timezone"
	RuleExpire   = "expire"
)

// RuleConfig is the JSON-serialisable description of a single rule. Only the
// fields relevant to the chosen Type are used.
type RuleConfig struct {
	Type string `json:"type"`

	// Enabled gates whether the rule runs. A nil pointer means enabled (so
	// configs written before this field existed keep working).
	Enabled *bool `json:"enabled,omitempty"`

	// Matching (filter, rename).
	MatchMode string `json:"matchMode,omitempty"` // "substring" | "regex"
	Pattern   string `json:"pattern,omitempty"`

	// filter
	FilterMode string   `json:"filterMode,omitempty"` // "blacklist" | "whitelist"
	Fields     []string `json:"fields,omitempty"`     // also used by strip

	// rename
	Field       string `json:"field,omitempty"`
	Replacement string `json:"replacement,omitempty"`

	// dedup
	KeyFields []string `json:"keyFields,omitempty"`

	// timezone
	Target    string `json:"target,omitempty"`
	DefaultTZ string `json:"defaultTz,omitempty"`

	// expire
	Days int `json:"days,omitempty"`
}

// BuildRule constructs a Rule from its configuration, validating parameters.
func BuildRule(c RuleConfig) (Rule, error) {
	switch c.Type {
	case RuleFilter:
		return NewFilterRule(FilterMode(c.FilterMode), MatchMode(c.MatchMode), c.Pattern, c.Fields)
	case RuleDedup:
		return NewDedupRule(c.KeyFields), nil
	case RuleRename:
		return NewRenameRule(c.Field, MatchMode(c.MatchMode), c.Pattern, c.Replacement)
	case RuleStrip:
		return NewStripRule(c.Fields)
	case RuleTimezone:
		return NewTimezoneRule(c.Target, c.DefaultTZ)
	case RuleExpire:
		return NewExpireRule(c.Days)
	default:
		return nil, fmt.Errorf("pipeline: unknown rule type %q", c.Type)
	}
}

// BuildPipeline constructs a Pipeline from an ordered list of rule configs.
func BuildPipeline(configs []RuleConfig) (*Pipeline, error) {
	rules := make([]Rule, 0, len(configs))
	for i, c := range configs {
		if c.Enabled != nil && !*c.Enabled {
			continue // rule explicitly disabled
		}
		r, err := BuildRule(c)
		if err != nil {
			return nil, fmt.Errorf("pipeline: rule %d: %w", i, err)
		}
		rules = append(rules, r)
	}
	return New(rules...), nil
}
