package attago

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataService_GetLatest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data/latest" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DataLatestResponse{AssetOrder: []string{"BTC", "ETH"}})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Data.GetLatest(context.Background())
	if err != nil {
		t.Fatalf("GetLatest() error: %v", err)
	}
	if len(result.AssetOrder) != 2 {
		t.Errorf("AssetOrder length = %d", len(result.AssetOrder))
	}
}

func TestDataService_GetTokenData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/data/BTC" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DataTokenResponse{
			Token:     "BTC",
			RequestID: "req-123",
			Mode:      "live",
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Data.GetTokenData(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("GetTokenData() error: %v", err)
	}
	if result.RequestID != "req-123" {
		t.Errorf("RequestID = %q", result.RequestID)
	}
}

func TestDataService_GetDataPush(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data/push/2026-03-07T12-00-00-000Z-abc" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DataPushResponse{
			RequestID: "2026-03-07T12-00-00-000Z-abc",
			TokenID:   "BTC",
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL))
	result, err := c.Data.GetDataPush(context.Background(), "2026-03-07T12-00-00-000Z-abc")
	if err != nil {
		t.Fatalf("GetDataPush() error: %v", err)
	}
	if result.TokenID != "BTC" {
		t.Errorf("TokenID = %q", result.TokenID)
	}
}
