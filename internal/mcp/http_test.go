package mcp

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestOriginPolicy(t *testing.T) {
	for _, tc := range []struct {
		origin, host   string
		https, allowed bool
	}{
		{"http://example.com", "example.com:80", false, true},
		{"https://example.com", "example.com:443", true, true},
		{"http://EXAMPLE.COM:80", "example.com", false, true},
		{"http://example.com:8080", "example.com", false, false},
		{"https://example.com", "example.com", false, false},
		{"http://example.com.evil", "example.com", false, false},
		{"https://trusted.example", "example.com", false, true},
		{"https://trusted.example/", "example.com", false, false},
		{"https://trusted.example#", "example.com", false, false},
		{"http://user@example.com", "example.com", false, false},
		{"null", "example.com", false, false},
	} {
		t.Run(tc.origin+"_"+tc.host, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://"+tc.host+"/mcp", nil)
			if tc.https {
				req.TLS = &tls.ConnectionState{}
			}
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Forwarded-Host", "trusted.example")
			if got := validOrigin(tc.origin, req, map[string]bool{"https://trusted.example": true}); got != tc.allowed {
				t.Fatalf("allowed=%v want %v", got, tc.allowed)
			}
		})
	}
}
