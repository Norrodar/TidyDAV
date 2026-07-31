package outbound

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
		why     string
	}{
		{"127.0.0.1", true, "loopback"},
		{"::1", true, "IPv6 loopback"},
		{"10.1.2.3", true, "private class A"},
		{"172.16.0.1", true, "private class B"},
		{"192.168.1.1", true, "private class C"},
		{"169.254.1.1", true, "link-local"},
		{"100.64.0.1", true, "CGNAT (RFC 6598)"},
		{"100.127.255.254", true, "CGNAT upper end"},
		{"0.0.0.0", true, "unspecified"},
		{"224.0.0.1", true, "multicast"},
		{"fd00::1", true, "unique local IPv6"},
		{"::ffff:10.0.0.1", true, "private IPv4 mapped into IPv6"},

		{"8.8.8.8", false, "public resolver"},
		{"1.1.1.1", false, "public resolver"},
		{"100.63.255.255", false, "just below CGNAT"},
		{"100.128.0.0", false, "just above CGNAT"},
		{"2606:4700:4700::1111", false, "public IPv6"},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("unparseable test address %q", tt.ip)
			}
			if got := IsBlockedIP(ip); got != tt.blocked {
				t.Errorf("IsBlockedIP(%s) = %v, want %v (%s)", tt.ip, got, tt.blocked, tt.why)
			}
		})
	}
}

// With private targets allowed the client reaches a loopback server; without,
// the very same request is refused before any bytes are exchanged.
func TestClientHonoursPrivateTargetPolicy(t *testing.T) {
	var reached bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	permissive := Client(5*time.Second, true)
	resp, err := permissive.Get(srv.URL)
	if err != nil {
		t.Fatalf("permissive client: %v", err)
	}
	_ = resp.Body.Close()
	if !reached {
		t.Fatal("permissive client did not reach the loopback server")
	}

	reached = false
	strict := Client(5*time.Second, false)
	if resp, err := strict.Get(srv.URL); err == nil {
		_ = resp.Body.Close()
		t.Fatal("strict client connected to a loopback address")
	}
	if reached {
		t.Error("strict client sent the request before refusing")
	}
}

func TestClientTimeoutIsApplied(t *testing.T) {
	if got := Client(7*time.Second, true).Timeout; got != 7*time.Second {
		t.Errorf("timeout = %v, want 7s", got)
	}
	if got := Client(7*time.Second, false).Timeout; got != 7*time.Second {
		t.Errorf("hardened timeout = %v, want 7s", got)
	}
}

func TestRedactURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// A secret-address calendar link is a bearer token in the query.
		{"https://cal.example.com/private/abc.ics?token=s3cr3t", "https://cal.example.com/private/abc.ics"},
		// A password pasted into the URL must never survive into a log line.
		{"https://user:hunter2@cal.example.com/x.ics", "https://user:xxxxx@cal.example.com/x.ics"},
		{"https://cal.example.com/x.ics#frag", "https://cal.example.com/x.ics"},
		{"https://cal.example.com/x.ics", "https://cal.example.com/x.ics"},
		{"://not a url", "[redacted]"},
	}
	for _, tc := range cases {
		if got := RedactURL(tc.in); got != tc.want {
			t.Errorf("RedactURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if strings.Contains(RedactURL(tc.in), "hunter2") || strings.Contains(RedactURL(tc.in), "s3cr3t") {
			t.Errorf("RedactURL(%q) leaked a secret", tc.in)
		}
	}
}

func TestRedactError(t *testing.T) {
	raw := "https://user:hunter2@cal.example.com/x.ics?token=s3cr3t"

	// Go's transport embeds the request URL verbatim.
	wrapped := &url.Error{Op: "Get", URL: raw, Err: errors.New("dial tcp: timeout")}
	got := RedactError(wrapped, raw)
	if strings.Contains(got.Error(), "hunter2") || strings.Contains(got.Error(), "s3cr3t") {
		t.Errorf("RedactError kept the credentials: %v", got)
	}

	// An error that does not carry the URL is unwrapped rather than reformatted.
	other := &url.Error{Op: "Get", URL: raw, Err: errors.New("connection refused")}
	if got := RedactError(other, "https://other.example.com"); got.Error() != "connection refused" {
		t.Errorf("RedactError = %q, want the unwrapped cause", got)
	}
	if RedactError(nil, raw) != nil {
		t.Error("RedactError(nil) is not nil")
	}
}
