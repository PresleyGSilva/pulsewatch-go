package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"pulsewatch-go/internal/ssrf"
)

type Spec struct {
	URL                string
	Method             string
	TimeoutMs          int
	ExpectStatus       int
	ExpectBodyContains string
}

type Result struct {
	StatusCode int
	OK         bool
	LatencyMs  int
	ErrMsg     string
}

var sem = make(chan struct{}, 8)

// Check performs a single HTTP check. Single-flight safe via global semaphore.
func Check(ctx context.Context, s Spec) Result {
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return Result{OK: false, ErrMsg: "check queue cancelled"}
	}
	if err := ssrf.ValidateURL(s.URL); err != nil {
		return Result{OK: false, ErrMsg: "ssrf blocked: " + err.Error()}
	}
	method := s.Method
	if method == "" {
		method = "GET"
	}
	timeout := time.Duration(s.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			prev := via[len(via)-1].URL.Hostname()
			next := req.URL.Hostname()
			if !equalHost(prev, next) {
				return fmt.Errorf("cross-host redirect blocked: %s", req.URL.String())
			}
			if err := ssrf.ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, s.URL, nil)
	if err != nil {
		return Result{OK: false, ErrMsg: err.Error()}
	}
	req.Header.Set("User-Agent", "PulseWatch/1.0")
	start := time.Now()
	resp, err := client.Do(req)
	lat := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{OK: false, LatencyMs: lat, ErrMsg: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	ok := matchStatus(resp.StatusCode, s.ExpectStatus)
	if ok && s.ExpectBodyContains != "" {
		ok = containsFold(string(body), s.ExpectBodyContains)
	}
	res := Result{StatusCode: resp.StatusCode, OK: ok, LatencyMs: lat}
	if !ok {
		res.ErrMsg = fmt.Sprintf("unexpected status %d", resp.StatusCode)
		if s.ExpectBodyContains != "" && !containsFold(string(body), s.ExpectBodyContains) {
			res.ErrMsg = "body assertion failed"
		}
	}
	return res
}

func matchStatus(got, want int) bool {
	if want == 0 {
		return got >= 200 && got < 300
	}
	return got == want
}

func containsFold(hay, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	// case-sensitive per spec (body contains assert); keep simple
	return len(hay) >= len(needle) && search(hay, needle)
}

func search(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func equalHost(a, b string) bool {
	if a == b {
		return true
	}
	// allow case-insensitive match
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		c1, c2 := a[i], b[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 32
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 32
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}
