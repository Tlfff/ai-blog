package requestip

import (
	"net/http/httptest"
	"testing"
)

// TestFromRequestTrustedProxy 验证只有受信代理才能提供转发客户端地址。
func TestFromRequestTrustedProxy(t *testing.T) {
	trusted := httptest.NewRequest("GET", "/", nil)
	trusted.RemoteAddr = "10.0.0.8:1234"
	trusted.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.8")
	if got := FromRequest(trusted, []string{"10.0.0.0/8"}); got != "203.0.113.9" {
		t.Fatalf("trusted IP = %q", got)
	}

	untrusted := httptest.NewRequest("GET", "/", nil)
	untrusted.RemoteAddr = "198.51.100.2:1234"
	untrusted.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := FromRequest(untrusted, []string{"10.0.0.0/8"}); got != "198.51.100.2" {
		t.Fatalf("untrusted IP = %q", got)
	}

	chain := httptest.NewRequest("GET", "/", nil)
	chain.RemoteAddr = "10.0.0.8:1234"
	chain.Header.Set("X-Forwarded-For", "198.51.100.7, 192.168.1.9, 10.0.0.7")
	if got := FromRequest(chain, []string{"10.0.0.0/8", "192.168.0.0/16"}); got != "198.51.100.7" {
		t.Fatalf("proxy chain IP = %q", got)
	}
}
