package pipeline

import "testing"

func TestMatcherSubstring(t *testing.T) {
	m, err := NewMatcher(MatchSubstring, "tonne")
	if err != nil {
		t.Fatalf("NewMatcher() error: %v", err)
	}
	if !m.MatchString("Schwarze Tonne") {
		t.Error("substring match should be case-insensitive")
	}
	if m.MatchString("Gelber Sack") {
		t.Error("unexpected match")
	}
}

func TestMatcherRegex(t *testing.T) {
	m, err := NewMatcher(MatchRegex, `^Bin (\d+)$`)
	if err != nil {
		t.Fatalf("NewMatcher() error: %v", err)
	}
	if !m.MatchString("Bin 42") {
		t.Error("regex should match")
	}
	if m.MatchString("Bin x") {
		t.Error("regex should not match")
	}
}

func TestMatcherInvalidRegex(t *testing.T) {
	if _, err := NewMatcher(MatchRegex, "("); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestMatcherUnknownMode(t *testing.T) {
	if _, err := NewMatcher(MatchMode("nope"), "x"); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}
