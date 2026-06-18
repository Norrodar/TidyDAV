package pipeline

import (
	"encoding/json"
	"testing"
)

func TestBuildPipelineFromJSON(t *testing.T) {
	raw := `[
		{"type":"filter","filterMode":"blacklist","matchMode":"substring","pattern":"spam"},
		{"type":"dedup","keyFields":["SUMMARY","DATE"]},
		{"type":"rename","field":"SUMMARY","matchMode":"regex","pattern":"Bin (\\d+)","replacement":"Trash $1"},
		{"type":"strip","fields":["DESCRIPTION"]},
		{"type":"timezone","target":"UTC","defaultTz":"Europe/Berlin"},
		{"type":"expire","days":30}
	]`
	var configs []RuleConfig
	if err := json.Unmarshal([]byte(raw), &configs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p, err := BuildPipeline(configs)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}
	if p.Len() != 6 {
		t.Fatalf("pipeline length = %d, want 6", p.Len())
	}
}

func TestBuildRuleErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  RuleConfig
	}{
		{"unknown type", RuleConfig{Type: "nope"}},
		{"bad filter mode", RuleConfig{Type: RuleFilter, FilterMode: "x", MatchMode: "substring", Pattern: "a"}},
		{"bad regex", RuleConfig{Type: RuleFilter, FilterMode: "blacklist", MatchMode: "regex", Pattern: "("}},
		{"rename bad target", RuleConfig{Type: RuleRename, Field: "DTSTART", MatchMode: "substring", Pattern: "a"}},
		{"strip no fields", RuleConfig{Type: RuleStrip}},
		{"timezone bad zone", RuleConfig{Type: RuleTimezone, Target: "Mars/Phobos"}},
		{"expire zero days", RuleConfig{Type: RuleExpire, Days: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildRule(tt.cfg); err == nil {
				t.Errorf("BuildRule(%+v) = nil error, want error", tt.cfg)
			}
		})
	}
}
