package attago

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── CRUD ────────────────────────────────────────────────────────────

func TestWebhookService_Create(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/user/webhooks" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["url"] != "https://example.com/hook" {
			t.Errorf("body.url = %q", body["url"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WebhookCreateResponse{
			WebhookID: "wh-123",
			URL:       "https://example.com/hook",
			Secret:    "wh_secret_abc",
			CreatedAt: "2026-03-08T00:00:00Z",
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Webhooks.Create(context.Background(), "https://example.com/hook")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if result.WebhookID != "wh-123" {
		t.Errorf("WebhookID = %q", result.WebhookID)
	}
	if result.Secret != "wh_secret_abc" {
		t.Errorf("Secret = %q", result.Secret)
	}
}

func TestWebhookService_List(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/user/webhooks" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": []WebhookListItem{
				{WebhookID: "wh-1", URL: "https://a.com/hook"},
				{WebhookID: "wh-2", URL: "https://b.com/hook"},
			},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	items, err := c.Webhooks.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len = %d, want 2", len(items))
	}
}

func TestWebhookService_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/v1/user/webhooks/wh-123" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	err := c.Webhooks.Delete(context.Background(), "wh-123")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
}

func TestWebhookService_SendServerTest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/user/webhooks/wh-123/test" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		sc := 200
		json.NewEncoder(w).Encode(WebhookTestResult{
			Success:    true,
			StatusCode: &sc,
			Attempts:   1,
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Webhooks.SendServerTest(context.Background(), "wh-123")
	if err != nil {
		t.Fatalf("SendServerTest() error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

// ── HMAC signing ────────────────────────────────────────────────────

func TestSignPayload_Deterministic(t *testing.T) {
	body := []byte(`{"event":"test","version":"2"}`)
	sig1 := SignPayload(body, "secret123")
	sig2 := SignPayload(body, "secret123")
	if sig1 != sig2 {
		t.Error("signatures should be deterministic")
	}
	if len(sig1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("sig length = %d, want 64", len(sig1))
	}
}

func TestSignPayload_DifferentSecrets(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	sig1 := SignPayload(body, "secret1")
	sig2 := SignPayload(body, "secret2")
	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestVerifySignature_Valid(t *testing.T) {
	body := []byte(`{"event":"test","version":"2"}`)
	secret := "wh_secret_test"
	sig := SignPayload(body, secret)

	if !VerifySignature(body, secret, sig) {
		t.Error("valid signature should verify")
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	if VerifySignature(body, "secret", "bad-signature") {
		t.Error("bad signature should not verify")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	sig := SignPayload(body, "secret1")
	if VerifySignature(body, "secret2", sig) {
		t.Error("wrong secret should not verify")
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	sig := SignPayload(body, "secret")
	tampered := []byte(`{"event":"alert"}`)
	if VerifySignature(tampered, "secret", sig) {
		t.Error("tampered body should not verify")
	}
}

// ── BuildTestPayload ────────────────────────────────────────────────

func TestBuildTestPayload_Defaults(t *testing.T) {
	p := BuildTestPayload("", "", "", "")
	if p["event"] != "test" {
		t.Errorf("event = %v", p["event"])
	}
	if p["version"] != "2" {
		t.Errorf("version = %v", p["version"])
	}
	if p["environment"] != "production" {
		t.Errorf("environment = %v", p["environment"])
	}
	alert := p["alert"].(map[string]any)
	if alert["token"] != "BTC" {
		t.Errorf("token = %v", alert["token"])
	}
	if alert["state"] != "triggered" {
		t.Errorf("state = %v", alert["state"])
	}
}

func TestBuildTestPayload_Custom(t *testing.T) {
	p := BuildTestPayload("ETH", "resolved", "staging", "staging.attago.io")
	alert := p["alert"].(map[string]any)
	if alert["token"] != "ETH" {
		t.Errorf("token = %v", alert["token"])
	}
	if alert["state"] != "resolved" {
		t.Errorf("state = %v", alert["state"])
	}
	if p["environment"] != "staging" {
		t.Errorf("environment = %v", p["environment"])
	}
	data := p["data"].(map[string]any)
	fallbackURL, ok := data["fallbackUrl"].(string)
	if !ok || fallbackURL != "https://staging.attago.io/data/latest.json" {
		t.Errorf("fallbackUrl = %v", data["fallbackUrl"])
	}
}

// ── SDK-side test delivery ──────────────────────────────────────────

func TestWebhookService_SendTest_Success(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-AttaGo-Signature")
		if sig == "" {
			t.Error("missing X-AttaGo-Signature")
		}
		event := r.Header.Get("X-AttaGo-Event")
		if event != "test" {
			t.Errorf("X-AttaGo-Event = %q", event)
		}

		body, _ := io.ReadAll(r.Body)
		if !VerifySignature(body, "test-secret", sig) {
			t.Error("signature verification failed")
		}

		w.WriteHeader(200)
	}))
	defer receiver.Close()

	c := NewClient()
	result, err := c.Webhooks.SendTest(context.Background(), SendTestOptions{
		URL:       receiver.URL,
		Secret:    "test-secret",
		BackoffMs: []int{0, 0, 0}, // no actual delays in tests
	})
	if err != nil {
		t.Fatalf("SendTest() error: %v", err)
	}
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
	if result.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", result.Attempts)
	}
}

func TestWebhookService_SendTest_PermanentFailure(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403) // 4xx = permanent failure
	}))
	defer receiver.Close()

	c := NewClient()
	result, err := c.Webhooks.SendTest(context.Background(), SendTestOptions{
		URL:       receiver.URL,
		Secret:    "test-secret",
		BackoffMs: []int{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("SendTest() error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for 403")
	}
	if result.Attempts != 1 {
		t.Errorf("attempts = %d, want 1 (permanent failure, no retry)", result.Attempts)
	}
}

func TestWebhookService_SendTest_RetryOn500(t *testing.T) {
	calls := 0
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(500) // retry
			return
		}
		w.WriteHeader(200) // success on 3rd attempt
	}))
	defer receiver.Close()

	c := NewClient()
	result, err := c.Webhooks.SendTest(context.Background(), SendTestOptions{
		URL:       receiver.URL,
		Secret:    "test-secret",
		BackoffMs: []int{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("SendTest() error: %v", err)
	}
	if !result.Success {
		t.Error("expected success on 3rd attempt")
	}
	if result.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", result.Attempts)
	}
}
