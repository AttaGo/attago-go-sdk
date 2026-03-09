package attago

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mustNewClient is a test helper that calls NewClient and fails the test on error.
func mustNewClient(t *testing.T, opts ...Option) *Client {
	t.Helper()
	c, err := NewClient(opts...)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	return c
}

// ── NewClient ───────────────────────────────────────────────────────

func TestNewClient_Defaults(t *testing.T) {
	c := mustNewClient(t)
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.AuthMode() != AuthModeNone {
		t.Errorf("AuthMode() = %q, want %q", c.AuthMode(), AuthModeNone)
	}
	if c.Auth != nil {
		t.Error("Auth should be nil for no-auth mode")
	}
}

func TestNewClient_WithAPIKey(t *testing.T) {
	c := mustNewClient(t, WithAPIKey("ak_live_test123"))
	if c.AuthMode() != AuthModeAPIKey {
		t.Errorf("AuthMode() = %q, want %q", c.AuthMode(), AuthModeAPIKey)
	}
	if c.apiKey != "ak_live_test123" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "ak_live_test123")
	}
}

func TestNewClient_WithBaseURL(t *testing.T) {
	c := mustNewClient(t, WithBaseURL("https://custom.example.com/"))
	if c.baseURL != "https://custom.example.com" {
		t.Errorf("baseURL = %q, want trailing slash stripped", c.baseURL)
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := mustNewClient(t, WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Error("httpClient should be the custom client")
	}
}

func TestNewClient_MultipleAuthModes_Error(t *testing.T) {
	_, err := NewClient(WithAPIKey("ak_live_x"), WithSigner(&mockSigner{}))
	if err == nil {
		t.Fatal("expected error for multiple auth modes")
	}
}

func TestNewClient_CognitoWithoutClientID_Error(t *testing.T) {
	_, err := NewClient(WithCognito("user@example.com", "pass", ""))
	if err == nil {
		t.Fatal("expected error for cognito without clientID")
	}
}

func TestNewClient_CognitoCreatesAuth(t *testing.T) {
	c := mustNewClient(t, WithCognito("user@example.com", "pass", "client-id-123"))
	if c.AuthMode() != AuthModeCognito {
		t.Errorf("AuthMode() = %q, want %q", c.AuthMode(), AuthModeCognito)
	}
	if c.Auth == nil {
		t.Fatal("Auth should not be nil in cognito mode")
	}
}

func TestNewClient_ServicesInitialized(t *testing.T) {
	c := mustNewClient(t)
	if c.Agent == nil {
		t.Error("Agent service should be initialized")
	}
	if c.Webhooks == nil {
		t.Error("Webhooks service should be initialized")
	}
	if c.MCP == nil {
		t.Error("MCP service should be initialized")
	}
	if c.Subscriptions == nil {
		t.Error("Subscriptions service should be initialized")
	}
	if c.Payments == nil {
		t.Error("Payments service should be initialized")
	}
	if c.Wallets == nil {
		t.Error("Wallets service should be initialized")
	}
	if c.APIKeys == nil {
		t.Error("APIKeys service should be initialized")
	}
	if c.Bundles == nil {
		t.Error("Bundles service should be initialized")
	}
	if c.Push == nil {
		t.Error("Push service should be initialized")
	}
	if c.Redeem == nil {
		t.Error("Redeem service should be initialized")
	}
	if c.Data == nil {
		t.Error("Data service should be initialized")
	}
}

// ── do() — request infrastructure ──────────────────────────────────

func TestDo_AddsV1Prefix(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/score" {
			t.Errorf("path = %q, want /v1/agent/score", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": "BTC"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	var result map[string]string
	err := c.do(context.Background(), "GET", "/agent/score", &result)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if result["token"] != "BTC" {
		t.Errorf("token = %q, want BTC", result["token"])
	}
}

func TestDo_PreservesV1Prefix(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/score" {
			t.Errorf("path = %q, want /v1/agent/score", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "GET", "/v1/agent/score", nil)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
}

func TestDo_SendsAPIKeyHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "ak_live_test" {
			t.Errorf("X-API-Key = %q, want ak_live_test", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_live_test"))
	err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
}

func TestDo_SendsUserAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		want := "attago-go-sdk/" + Version
		if ua != want {
			t.Errorf("User-Agent = %q, want %q", ua, want)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
}

func TestDo_SendsJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["url"] != "https://example.com" {
			t.Errorf("body.url = %q, want https://example.com", body["url"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "POST", "/test", nil, WithBody(map[string]string{"url": "https://example.com"}))
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
}

func TestDo_QueryParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("symbol"); got != "BTC" {
			t.Errorf("query symbol = %q, want BTC", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "GET", "/test", nil, WithQuery("symbol", "BTC"))
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
}

func TestDo_204_NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "DELETE", "/test", nil)
	if err != nil {
		t.Fatalf("do() error on 204: %v", err)
	}
}

// ── Error handling ──────────────────────────────────────────────────

func TestDo_404_ReturnsAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Not found"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "Not found" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Not found")
	}
}

func TestDo_429_ReturnsRateLimitError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
		json.NewEncoder(w).Encode(map[string]string{"error": "Too many requests"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *RateLimitError, got %T", err)
	}
	if rlErr.RetryAfter != 120 {
		t.Errorf("RetryAfter = %d, want 120", rlErr.RetryAfter)
	}
}

func TestDo_402_ReturnsPaymentRequiredError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(map[string]string{"error": "Payment required"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError, got %T", err)
	}
}

func TestDo_500_ReturnsAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal server error"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	err := c.do(context.Background(), "GET", "/test", nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
}

// ── Context cancellation ────────────────────────────────────────────

func TestDo_CancelledContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := c.do(ctx, "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ── Mock signer for tests ───────────────────────────────────────────

type mockSigner struct {
	address   string
	network   string
	signFn    func(ctx context.Context, reqs *X402PaymentRequirements) (string, error)
}

func (m *mockSigner) Address() string { return m.address }
func (m *mockSigner) Network() string { return m.network }
func (m *mockSigner) Sign(ctx context.Context, reqs *X402PaymentRequirements) (string, error) {
	if m.signFn != nil {
		return m.signFn(ctx, reqs)
	}
	return "mock-signature", nil
}
