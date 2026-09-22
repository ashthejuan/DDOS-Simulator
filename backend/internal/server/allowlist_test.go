package server

import (
	"testing"
)

func TestAllowedTarget(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// Allowed: loopback + compose service name.
		{"localhost", "http://localhost:8081/api/test", true},
		{"localhost no port", "http://localhost/api/test", true},
		{"loopback ip", "http://127.0.0.1:8081/api/test", true},
		{"ipv6 loopback", "http://[::1]:8081/api/test", true},
		{"compose service", "http://test-server:8081/api/test", true},
		{"uppercase host", "http://LOCALHOST:8081/api/test", true},

		// Rejected: external hosts.
		{"external https", "https://example.com/", false},
		{"external http", "http://example.com/", false},
		{"external ip", "http://93.184.216.34/", false},

		// Rejected: lookalike / suffix tricks.
		{"suffix trick", "http://localhost.evil.com/", false},
		{"suffix trick ip", "http://127.0.0.1.evil.com/", false},
		{"prefix trick", "http://evillocalhost:8081/", false},

		// Rejected: scheme / credential tricks.
		{"https to localhost", "https://localhost:8081/api/test", false},
		{"userinfo trick", "http://localhost@evil.com/", false},
		{"userinfo on allowed", "http://user@localhost:8081/", false},
		{"file scheme", "file:///etc/passwd", false},
		{"no scheme", "localhost:8081/api/test", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AllowedTarget(tt.url, DefaultAllowedHosts); got != tt.want {
				t.Fatalf("AllowedTarget(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestAllowedHostsFromEnv(t *testing.T) {
	t.Setenv("ALLOWED_TARGETS", " test-server , 127.0.0.1 ")
	got := AllowedHostsFromEnv()
	if len(got) != 2 || got[0] != "test-server" || got[1] != "127.0.0.1" {
		t.Fatalf("got %v", got)
	}
	if !AllowedTarget("http://test-server:8081/x", got) {
		t.Fatal("custom allowlist should permit test-server")
	}
	if AllowedTarget("http://localhost:8081/x", got) {
		t.Fatal("custom allowlist should drop localhost")
	}
}
