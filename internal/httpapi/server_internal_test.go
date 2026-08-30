package httpapi

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIPIgnoresForwardingHeadersUnlessConfigured(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/auth/login", nil)
	request.RemoteAddr = "192.0.2.10:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.20, 192.0.2.10")
	server := &Server{}
	if got := server.clientIP(request); got != "192.0.2.10" {
		t.Fatalf("untrusted forwarded address used: %q", got)
	}
	server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	if got := server.clientIP(request); got != "192.0.2.10" {
		t.Fatalf("header from an untrusted peer was used: %q", got)
	}
	server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	if got := server.clientIP(request); got != "203.0.113.20" {
		t.Fatalf("header from a trusted proxy was ignored: %q", got)
	}
	request.Header.Set("X-Forwarded-For", "198.51.100.99, 203.0.113.20")
	if got := server.clientIP(request); got != "203.0.113.20" {
		t.Fatalf("spoofed prefix in an appended forwarding chain was trusted: %q", got)
	}
}

func TestLoginLimiterBoundsDistinctClientEntries(t *testing.T) {
	limiter := newLoginLimiter()
	limiter.maxEntries = 2
	for _, address := range []string{"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4"} {
		if !limiter.allow(address) {
			t.Fatalf("first attempt for %s was unexpectedly rejected", address)
		}
	}
	if len(limiter.attempts) > limiter.maxEntries {
		t.Fatalf("limiter retained %d entries with maximum %d", len(limiter.attempts), limiter.maxEntries)
	}
	if _, exists := limiter.attempts[loginLimitOverflow]; !exists {
		t.Fatal("overflow clients were not grouped into a bounded bucket")
	}
}
