// Package outbound builds the HTTP clients TidyDAV uses to reach servers a user
// configured (calendar sources, DAV endpoints, notification targets).
//
// Every such request is attacker-influenced by definition, so they all share one
// policy: when private targets are disallowed, the client refuses to connect to
// loopback, private, link-local and CGNAT addresses.
package outbound

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client returns an HTTP client with the given timeout. When allowPrivate is
// false it refuses non-public destinations (SSRF hardening for multi-user or
// internet-facing instances).
func Client(timeout time.Duration, allowPrivate bool) *http.Client {
	if allowPrivate {
		return &http.Client{Timeout: timeout}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// Resolve and validate the target before dialing, then dial the
			// checked IP so a DNS rebind cannot slip a private address through.
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, ip := range ips {
					if IsBlockedIP(ip.IP) {
						return nil, fmt.Errorf("refusing to connect to non-public address %s", ip.IP)
					}
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
			},
		},
	}
}

// cgnat is the carrier-grade NAT range (RFC 6598), which net.IP.IsPrivate does
// not cover but which addresses internal hosts in many cloud setups.
var cgnat = net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// IsBlockedIP reports whether an address is off-limits for user-configured
// destinations.
func IsBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() ||
		cgnat.Contains(ip)
}

// RedactURL reduces a user-configured URL to what is safe to log or forward:
// the query string is dropped and any userinfo password is masked.
//
// Both halves carry secrets in practice. A "secret address" calendar link
// (iCloud, Google, Nextcloud share) is a bearer token in the query, and a URL
// pasted as https://user:pass@host/cal.ics carries the password in the
// userinfo. Neither belongs in a log file or in a notification delivered to a
// third-party server. What remains — scheme, host, path — is enough to tell two
// sources apart while debugging.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[redacted]"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.Redacted()
}

// RedactError removes the raw URL from an error message. Go's transport errors
// embed the request URL verbatim (`Get "https://user:pass@host/x": …`), so
// redacting only the URL an error is logged next to would not help.
func RedactError(err error, raw string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if raw == "" || !strings.Contains(msg, raw) {
		// Unwrap the URL error so its own formatting (which re-adds the URL)
		// is not reused.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return uerr.Err
		}
		return err
	}
	return errors.New(strings.ReplaceAll(msg, raw, RedactURL(raw)))
}
