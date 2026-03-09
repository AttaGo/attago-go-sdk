package attago

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WebhookService provides access to webhook management and test delivery.
type WebhookService struct {
	client *Client
}

// Create registers a new webhook URL. Requires Pro+ tier.
// The Secret in the response is shown only once — save it immediately.
func (s *WebhookService) Create(ctx context.Context, webhookURL string) (*WebhookCreateResponse, error) {
	var result WebhookCreateResponse
	err := s.client.do(ctx, "POST", "/user/webhooks", &result, WithBody(map[string]string{"url": webhookURL}))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns registered webhooks (secrets stripped).
func (s *WebhookService) List(ctx context.Context) ([]WebhookListItem, error) {
	var result struct {
		Items []WebhookListItem `json:"items"`
	}
	err := s.client.do(ctx, "GET", "/user/webhooks", &result)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Delete removes a webhook by ID.
func (s *WebhookService) Delete(ctx context.Context, webhookID string) error {
	return s.client.do(ctx, "DELETE", "/user/webhooks/"+url.PathEscape(webhookID), nil)
}

// SendTest sends a test webhook from the client machine.
// Builds a v2 test payload, HMAC-signs it, and POSTs to the given URL with retry.
func (s *WebhookService) SendTest(ctx context.Context, opts SendTestOptions) (*WebhookTestResult, error) {
	payload := BuildTestPayload(opts.Token, opts.State, opts.Environment, "")
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("attago: marshal test payload: %w", err)
	}

	signature := SignPayload(bodyBytes, opts.Secret)

	headers := map[string]string{
		"Content-Type":       "application/json",
		"X-AttaGo-Signature": signature,
		"X-AttaGo-Event":     "test",
		"User-Agent":         "AttaGo-Webhook/1.0",
	}

	backoff := opts.BackoffMs
	if len(backoff) == 0 {
		backoff = []int{1000, 4000, 16000}
	}

	return deliverWithRetry(ctx, s.client.httpClient, opts.URL, bodyBytes, headers, backoff)
}

// SendServerTest triggers server-side test delivery for a registered webhook.
func (s *WebhookService) SendServerTest(ctx context.Context, webhookID string) (*WebhookTestResult, error) {
	var result WebhookTestResult
	err := s.client.do(ctx, "POST", "/user/webhooks/"+url.PathEscape(webhookID)+"/test", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Exported helpers ────────────────────────────────────────────────

// BuildTestPayload builds a v2 test webhook payload.
func BuildTestPayload(token, state, environment, domain string) map[string]any {
	if token == "" {
		token = "BTC"
	}
	if state == "" {
		state = "triggered"
	}
	if environment == "" {
		environment = "production"
	}
	if domain == "" {
		domain = "attago.bid"
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	expires := time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339Nano)

	return map[string]any{
		"event":       "test",
		"version":     "2",
		"environment": environment,
		"alert": map[string]any{
			"id":    "test",
			"label": "Test webhook delivery",
			"token": token,
			"state": state,
		},
		"data": map[string]any{
			"url":         nil,
			"expiresAt":   expires,
			"fallbackUrl": "https://" + domain + "/data/latest.json",
		},
		"timestamp": now,
	}
}

// SignPayload HMAC-SHA256 signs a payload with the given secret. Returns hex digest.
func SignPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies an incoming webhook signature using timing-safe comparison.
func VerifySignature(rawBody []byte, secret, signature string) bool {
	expected := SignPayload(rawBody, secret)
	if len(expected) != len(signature) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

// ── Retry logic ─────────────────────────────────────────────────────

const requestTimeoutMs = 5000

func deliverWithRetry(ctx context.Context, hc *http.Client, targetURL string, body []byte, headers map[string]string, backoffMs []int) (*WebhookTestResult, error) {
	var lastStatusCode *int
	var lastError string

	maxAttempts := len(backoffMs) + 1

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(backoffMs[attempt-1]) * time.Millisecond
			select {
			case <-ctx.Done():
				return &WebhookTestResult{
					Success:    false,
					StatusCode: lastStatusCode,
					Attempts:   attempt,
					Error:      "context cancelled",
				}, nil
			case <-time.After(delay):
			}
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(requestTimeoutMs)*time.Millisecond)

		req, err := http.NewRequestWithContext(timeoutCtx, "POST", targetURL, io.NopCloser(newBytesReader(body)))
		if err != nil {
			cancel()
			lastError = err.Error()
			continue
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		res, err := hc.Do(req)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				lastError = "timeout"
			} else {
				lastError = err.Error()
			}
			continue
		}
		_ = res.Body.Close()

		sc := res.StatusCode
		lastStatusCode = &sc

		if sc >= 200 && sc < 300 {
			return &WebhookTestResult{
				Success:    true,
				StatusCode: &sc,
				Attempts:   attempt + 1,
			}, nil
		}

		// 4xx (except 429) = permanent failure, no retry
		if sc >= 400 && sc < 500 && sc != 429 {
			return &WebhookTestResult{
				Success:    false,
				StatusCode: &sc,
				Attempts:   attempt + 1,
				Error:      fmt.Sprintf("HTTP %d", sc),
			}, nil
		}

		lastError = fmt.Sprintf("HTTP %d", sc)
	}

	return &WebhookTestResult{
		Success:    false,
		StatusCode: lastStatusCode,
		Attempts:   maxAttempts,
		Error:      lastError,
	}, nil
}

// newBytesReader is a helper so we don't import bytes in this file just for this.
func newBytesReader(b []byte) io.Reader {
	return &bytesReader{data: b, pos: 0}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
