package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// SendWebhook POSTs JSON payload with 5s timeout. Never fails the check.
func SendWebhook(ctx context.Context, target string, payload any) string {
	if target == "" {
		return ""
	}
	body, _ := json.Marshal(payload)
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return "webhook request build error: " + err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PulseWatch/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "webhook error: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "webhook status: " + resp.Status
	}
	return ""
}
