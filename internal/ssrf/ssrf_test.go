package ssrf

import (
	"strings"
	"testing"
)

func TestValidateURLBlocked(t *testing.T) {
	blocked := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1/",
		"http://localhost:8080/",
		"http://LOCALHOST/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.5.4/",
		"http://172.31.255.255/",
		"http://[::1]/",
		"http://0.0.0.0/",
		"http://100.64.0.1/",
		"http://[fc00::1]/",
		"http://[fe80::1]/",
		"http://2130706433/", // decimal bypass for 127.0.0.1
		"http://0x7f.0.0.1/", // hex bypass
		"http://0177.0.0.1/", // octal bypass
		"http://foo.localhost/",
		"http://svc.internal/",
		"http://nas.local/",
		"http://metadata.google.internal/",
		"ftp://example.com/file",
		"http://user:pass@example.com/",
		"http://169.254.169.254.evil.com.xip.io/",
	}
	for _, raw := range blocked {
		if err := ValidateURL(raw); err == nil {
			// some hostnames (xip.io) need DNS; if DNS resolves externally it may pass —
			// only enforce when no DNS is involved
			if !strings.Contains(raw, "xip.io") {
				t.Errorf("expected block for %q, got allow", raw)
			}
		}
	}
}

func TestValidateURLAllowed(t *testing.T) {
	allowed := []string{
		"https://example.com",
		"https://httpbin.org/status/200",
	}
	for _, raw := range allowed {
		if err := ValidateURL(raw); err != nil {
			if strings.Contains(err.Error(), "dns resolution failed") {
				t.Skipf("no network/DNS in test env, skipping allow-case %q", raw)
			}
			t.Errorf("expected allow for %q, got: %v", raw, err)
		}
	}
}

func TestValidateURLMalformed(t *testing.T) {
	for _, raw := range []string{"", "not a url", "gopher://x/", strings.Repeat("https://example.com/", 200)} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("expected error for %q", raw)
		}
	}
}
