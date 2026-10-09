package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCheckBlocksLocalTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	res := Check(context.Background(), Spec{URL: srv.URL, TimeoutMs: 3000, ExpectStatus: 200})
	if res.OK {
		t.Fatal("local httptest target must be blocked by SSRF guard")
	}
	if !strings.Contains(res.ErrMsg, "ssrf blocked") {
		t.Fatalf("expected ssrf block message, got %q", res.ErrMsg)
	}
}

func TestEqualHost(t *testing.T) {
	if !equalHost("example.com", "EXAMPLE.com") {
		t.Fatal("host compare should be case-insensitive")
	}
	if equalHost("example.com", "evil.com") {
		t.Fatal("different hosts must not match")
	}
	if equalHost("example.com", "sub.example.com") {
		t.Fatal("subdomain must not match (redirect policy)")
	}
}

func TestMatchStatus(t *testing.T) {
	if !matchStatus(201, 0) || matchStatus(500, 0) {
		t.Fatal("want=0 should accept any 2xx")
	}
	if !matchStatus(200, 200) || matchStatus(404, 200) {
		t.Fatal("exact status match broken")
	}
}

func TestDBIntegrationSkipped(t *testing.T) {
	if v := envOrEmpty("DATABASE_URL"); v == "" {
		t.Skip("DATABASE_URL unset, skipping DB integration")
	}
}

func envOrEmpty(k string) string {
	return os.Getenv(k)
}
