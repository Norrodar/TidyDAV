// Package outbound builds the HTTP clients TidyDAV uses to reach servers a user
// configured (calendar sources, DAV endpoints, notification targets).
//
// Every such request is attacker-influenced by definition, so they all share one
// policy: when private targets are disallowed, the client refuses to connect to
// loopback, private, link-local and CGNAT addresses.
package outbound

import (
	"context"
	"fmt"
	"net"
	"net/http"
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
