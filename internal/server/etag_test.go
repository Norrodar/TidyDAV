package server

import "testing"

func TestEntityTagIsContentDerived(t *testing.T) {
	body := []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")
	tag := entityTag(body)
	if tag != entityTag(body) {
		t.Errorf("entityTag is not deterministic: %q", tag)
	}
	if len(tag) < 3 || tag[0] != '"' || tag[len(tag)-1] != '"' {
		t.Errorf("entityTag = %q, want a quoted strong tag", tag)
	}
	if tag == entityTag([]byte("BEGIN:VCALENDAR\r\nX:1\r\nEND:VCALENDAR\r\n")) {
		t.Error("different bodies produced the same entity tag")
	}
}

func TestETagMatches(t *testing.T) {
	const etag = `"abc123"`
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"absent header", "", false},
		{"blank header", "   ", false},
		{"wildcard", "*", true},
		{"exact", `"abc123"`, true},
		{"padded", `  "abc123"  `, true},
		{"weak client tag", `W/"abc123"`, true},
		{"list contains it", `"other", "abc123"`, true},
		{"list without it", `"other", "nope"`, false},
		{"foreign tag", `"def456"`, false},
		{"unquoted garbage", `abc123`, false},
		{"substring only", `"abc"`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := etagMatches(tc.header, etag); got != tc.want {
				t.Errorf("etagMatches(%q, %q) = %v, want %v", tc.header, etag, got, tc.want)
			}
		})
	}
}

// A weak tag on our side must still match what the client echoes back.
func TestETagMatchesWeakServerTag(t *testing.T) {
	if !etagMatches(`"abc123"`, `W/"abc123"`) {
		t.Error("weak server tag did not match the strong tag the client sent")
	}
}
