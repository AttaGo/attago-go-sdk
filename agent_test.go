package attago

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentService_GetScore(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/score" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("symbol") != "BTC" {
			t.Errorf("symbol = %q", r.URL.Query().Get("symbol"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentScoreResponse{
			Token:     "BTC",
			Composite: CompositeScore{Score: 72, Signal: "GO", Confidence: 0.85},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Agent.GetScore(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("GetScore() error: %v", err)
	}
	if result.Token != "BTC" {
		t.Errorf("Token = %q", result.Token)
	}
	if result.Composite.Signal != "GO" {
		t.Errorf("Signal = %q", result.Composite.Signal)
	}
	if result.Composite.Score != 72 {
		t.Errorf("Score = %f", result.Composite.Score)
	}
}

func TestAgentService_GetData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/data" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("symbols"); got != "BTC,ETH" {
			t.Errorf("symbols = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentDataResponse{
			AssetOrder: []string{"BTC", "ETH"},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Agent.GetData(context.Background(), "BTC", "ETH")
	if err != nil {
		t.Fatalf("GetData() error: %v", err)
	}
	if len(result.AssetOrder) != 2 {
		t.Errorf("AssetOrder length = %d", len(result.AssetOrder))
	}
}

func TestAgentService_GetData_NoSymbols(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("symbols") != "" {
			t.Errorf("symbols should be empty, got %q", r.URL.Query().Get("symbols"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentDataResponse{AssetOrder: []string{"BTC"}})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	_, err := c.Agent.GetData(context.Background())
	if err != nil {
		t.Fatalf("GetData() error: %v", err)
	}
}
