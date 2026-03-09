package attago

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── ParsePaymentRequired ────────────────────────────────────────────

func TestParsePaymentRequired_Valid(t *testing.T) {
	reqs := &X402PaymentRequirements{
		X402Version: 1,
		Resource:    X402Resource{URL: "https://api.attago.bid/v1/agent/score", Description: "score", MimeType: "application/json"},
		Accepts: []X402AcceptedPayment{
			{Scheme: "exact", Network: "eip155:8453", Amount: "100000", Asset: "USDC", PayTo: "0xabc", MaxTimeoutSeconds: 30},
		},
	}
	encoded, _ := json.Marshal(reqs)
	b64 := base64.StdEncoding.EncodeToString(encoded)

	headers := http.Header{}
	headers.Set("Payment-Required", b64)

	result := ParsePaymentRequired(headers)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.X402Version != 1 {
		t.Errorf("X402Version = %d, want 1", result.X402Version)
	}
	if len(result.Accepts) != 1 {
		t.Fatalf("len(Accepts) = %d, want 1", len(result.Accepts))
	}
	if result.Accepts[0].Network != "eip155:8453" {
		t.Errorf("Network = %q, want eip155:8453", result.Accepts[0].Network)
	}
}

func TestParsePaymentRequired_MissingHeader(t *testing.T) {
	result := ParsePaymentRequired(http.Header{})
	if result != nil {
		t.Error("expected nil for missing header")
	}
}

func TestParsePaymentRequired_InvalidBase64(t *testing.T) {
	headers := http.Header{}
	headers.Set("Payment-Required", "not-valid-base64!!!")
	result := ParsePaymentRequired(headers)
	if result != nil {
		t.Error("expected nil for invalid base64")
	}
}

func TestParsePaymentRequired_InvalidJSON(t *testing.T) {
	headers := http.Header{}
	headers.Set("Payment-Required", base64.StdEncoding.EncodeToString([]byte("not json")))
	result := ParsePaymentRequired(headers)
	if result != nil {
		t.Error("expected nil for invalid JSON")
	}
}

// ── FilterAcceptsByNetwork ──────────────────────────────────────────

func TestFilterAcceptsByNetwork_Found(t *testing.T) {
	accepts := []X402AcceptedPayment{
		{Network: "eip155:8453", Amount: "100000"},
		{Network: "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", Amount: "200000"},
	}
	result := FilterAcceptsByNetwork(accepts, "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp")
	if result == nil {
		t.Fatal("expected match")
	}
	if result.Amount != "200000" {
		t.Errorf("Amount = %q, want 200000", result.Amount)
	}
}

func TestFilterAcceptsByNetwork_NotFound(t *testing.T) {
	accepts := []X402AcceptedPayment{
		{Network: "eip155:8453"},
	}
	result := FilterAcceptsByNetwork(accepts, "eip155:137")
	if result != nil {
		t.Error("expected nil for no match")
	}
}

func TestFilterAcceptsByNetwork_Empty(t *testing.T) {
	result := FilterAcceptsByNetwork(nil, "eip155:8453")
	if result != nil {
		t.Error("expected nil for empty accepts")
	}
}

// ── doWithX402 ──────────────────────────────────────────────────────

func TestDoWithX402_NonPaymentResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": "BTC"})
	}))
	defer ts.Close()

	c := mustNewClient(t,
		WithBaseURL(ts.URL),
		WithSigner(&mockSigner{network: "eip155:8453"}),
	)
	result, err := c.Agent.GetScore(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token != "BTC" {
		t.Errorf("Token = %q, want BTC", result.Token)
	}
}

func TestDoWithX402_AutoRetryOnPayment(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// First call: 402 with payment requirements
			reqs := &X402PaymentRequirements{
				X402Version: 1,
				Resource:    X402Resource{URL: "test"},
				Accepts: []X402AcceptedPayment{
					{Scheme: "exact", Network: "eip155:8453", Amount: "100000", PayTo: "0xabc", MaxTimeoutSeconds: 30},
				},
			}
			encoded, _ := json.Marshal(reqs)
			w.Header().Set("Payment-Required", base64.StdEncoding.EncodeToString(encoded))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(402)
			json.NewEncoder(w).Encode(map[string]string{"error": "Payment required"})
			return
		}

		// Second call: verify payment signature header
		sig := r.Header.Get("Payment-Signature")
		if sig != "mock-signed-payment" {
			t.Errorf("Payment-Signature = %q, want mock-signed-payment", sig)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": "BTC"})
	}))
	defer ts.Close()

	signer := &mockSigner{
		network: "eip155:8453",
		signFn: func(_ context.Context, _ *X402PaymentRequirements) (string, error) {
			return "mock-signed-payment", nil
		},
	}

	c := mustNewClient(t, WithBaseURL(ts.URL), WithSigner(signer))
	result, err := c.Agent.GetScore(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token != "BTC" {
		t.Errorf("Token = %q, want BTC", result.Token)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (initial + retry)", calls)
	}
}

func TestDoWithX402_NoMatchingNetwork(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reqs := &X402PaymentRequirements{
			X402Version: 1,
			Resource:    X402Resource{URL: "test"},
			Accepts: []X402AcceptedPayment{
				{Network: "eip155:137", Amount: "100000"}, // Polygon, not Base
			},
		}
		encoded, _ := json.Marshal(reqs)
		w.Header().Set("Payment-Required", base64.StdEncoding.EncodeToString(encoded))
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(map[string]string{"error": "Payment required"})
	}))
	defer ts.Close()

	signer := &mockSigner{network: "eip155:8453"} // Base
	c := mustNewClient(t, WithBaseURL(ts.URL), WithSigner(signer))

	_, err := c.Agent.GetScore(context.Background(), "BTC")
	if err == nil {
		t.Fatal("expected error for network mismatch")
	}

	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError, got %T: %v", err, err)
	}
}

func TestDoWithX402_SignerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reqs := &X402PaymentRequirements{
			X402Version: 1,
			Accepts:     []X402AcceptedPayment{{Network: "eip155:8453"}},
		}
		encoded, _ := json.Marshal(reqs)
		w.Header().Set("Payment-Required", base64.StdEncoding.EncodeToString(encoded))
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer ts.Close()

	signer := &mockSigner{
		network: "eip155:8453",
		signFn: func(_ context.Context, _ *X402PaymentRequirements) (string, error) {
			return "", errors.New("wallet locked")
		},
	}
	c := mustNewClient(t, WithBaseURL(ts.URL), WithSigner(signer))

	_, err := c.Agent.GetScore(context.Background(), "BTC")
	if err == nil {
		t.Fatal("expected error for signer failure")
	}
	if !containsString(err.Error(), "wallet locked") {
		t.Errorf("error = %q, want to contain 'wallet locked'", err.Error())
	}
}

func TestDoWithX402_RetryAlso402(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reqs := &X402PaymentRequirements{
			X402Version: 1,
			Accepts:     []X402AcceptedPayment{{Network: "eip155:8453"}},
		}
		encoded, _ := json.Marshal(reqs)
		w.Header().Set("Payment-Required", base64.StdEncoding.EncodeToString(encoded))
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient funds"})
	}))
	defer ts.Close()

	signer := &mockSigner{network: "eip155:8453"}
	c := mustNewClient(t, WithBaseURL(ts.URL), WithSigner(signer))

	_, err := c.Agent.GetScore(context.Background(), "BTC")
	if err == nil {
		t.Fatal("expected error when retry also returns 402")
	}

	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError, got %T", err)
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
