package redirect

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name, remote, xff, want string
	}{
		{"no proxy", "192.168.1.5:54321", "", "192.168.1.5"},
		{"ipv6 no proxy", "[2001:db8::1]:54321", "", "2001:db8::1"},
		{"behind nginx", "10.0.0.1:40000", "203.0.113.5", "203.0.113.5"},
		{"proxy chain", "10.0.0.1:40000", "203.0.113.5, 10.0.0.1", "203.0.113.5"},
		{"xff with spaces", "10.0.0.1:40000", "  203.0.113.5 , 10.0.0.1", "203.0.113.5"},
		{"empty xff falls back", "192.168.1.5:54321", "", "192.168.1.5"},
		{"malformed remote", "not-an-addr", "", "not-an-addr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/abc12345", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}

			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
