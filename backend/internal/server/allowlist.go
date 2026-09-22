// Allowlist enforcement for experiment targets (Phase 1).
//
// The simulator must only ever hit the bundled local test server.
// Targets are validated against an explicit host allowlist — the
// frontend never gets an arbitrary-URL field and the backend
// re-validates rather than trusting callers.
package server

import (
	"net/url"
	"os"
	"strings"
)

// DefaultAllowedHosts is the V1 allowlist: loopback + compose service name.
var DefaultAllowedHosts = []string{"localhost", "127.0.0.1", "::1", "test-server"}

// AllowedTarget reports whether rawURL is a permitted experiment target.
// Rules: http scheme only, no userinfo, hostname (case-insensitive,
// port-agnostic) must exactly match an entry in allowedHosts.
func AllowedTarget(rawURL string, allowedHosts []string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" {
		return false
	}
	if u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, h := range allowedHosts {
		if host == h {
			return true
		}
	}
	return false
}

// AllowedHostsFromEnv returns the allowlist, overridable via a
// comma-separated ALLOWED_TARGETS env var (defaults to DefaultAllowedHosts).
func AllowedHostsFromEnv() []string {
	raw := os.Getenv("ALLOWED_TARGETS")
	if raw == "" {
		return DefaultAllowedHosts
	}
	var hosts []string
	for _, h := range strings.Split(raw, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return DefaultAllowedHosts
	}
	return hosts
}
