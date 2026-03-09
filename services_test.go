package attago

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── SubscriptionService ─────────────────────────────────────────────

func TestSubscriptionService_Catalog(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/subscriptions/catalog" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CatalogResponse{
			Tokens:           []string{"BTC", "ETH"},
			Tier:             "pro",
			MaxSubscriptions: 50,
			Mode:             "full",
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Subscriptions.Catalog(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.MaxSubscriptions != 50 {
		t.Errorf("MaxSubscriptions = %d", result.MaxSubscriptions)
	}
}

func TestSubscriptionService_List(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/user/subscriptions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"subscriptions": []Subscription{{SubID: "sub-1", TokenID: "BTC"}},
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	subs, err := c.Subscriptions.List(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(subs) != 1 || subs[0].SubID != "sub-1" {
		t.Errorf("unexpected subs: %v", subs)
	}
}

func TestSubscriptionService_Create(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Subscription{SubID: "BTC#spot_score", TokenID: "BTC"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{
		TokenID: "BTC",
		Label:   "Test alert",
		Groups:  [][]SubscriptionCondition{{{MetricName: "spot_score", ThresholdOp: "gt", ThresholdVal: 70}}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.SubID != "BTC#spot_score" {
		t.Errorf("SubID = %q", result.SubID)
	}
}

func TestSubscriptionService_Update(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/v1/user/subscriptions/sub-1" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Subscription{SubID: "sub-1"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	newLabel := "Updated"
	_, err := c.Subscriptions.Update(context.Background(), "sub-1", UpdateSubscriptionInput{Label: &newLabel})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestSubscriptionService_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/v1/user/subscriptions/sub-1" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	err := c.Subscriptions.Delete(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

// ── PaymentService ──────────────────────────────────────────────────

func TestPaymentService_Subscribe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/payments/subscribe" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SubscribeResponse{Tier: "pro", Price: 29.99})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Payments.Subscribe(context.Background(), SubscribeInput{Tier: "pro", BillingCycle: "monthly"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Tier != "pro" {
		t.Errorf("Tier = %q", result.Tier)
	}
}

func TestPaymentService_Status(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payments/status" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BillingStatus{Tier: "free", MaxSubs: 3})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Payments.Status(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.MaxSubs != 3 {
		t.Errorf("MaxSubs = %d", result.MaxSubs)
	}
}

func TestPaymentService_UpgradeQuote(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tier") != "business" {
			t.Errorf("tier = %q", r.URL.Query().Get("tier"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(UpgradeQuote{TargetTier: "business", FinalPrice: 49.99})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Payments.UpgradeQuote(context.Background(), "business", "monthly")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.FinalPrice != 49.99 {
		t.Errorf("FinalPrice = %f", result.FinalPrice)
	}
}

// ── WalletService ───────────────────────────────────────────────────

func TestWalletService_Register(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/payments/wallet" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Wallet{WalletAddress: "0xabc", Chain: "base"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Wallets.Register(context.Background(), RegisterWalletInput{
		WalletAddress: "0xabc", Chain: "base", Signature: "sig", Timestamp: 1709856000,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Chain != "base" {
		t.Errorf("Chain = %q", result.Chain)
	}
}

func TestWalletService_List(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"wallets": []Wallet{{WalletAddress: "0x1"}, {WalletAddress: "0x2"}},
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	wallets, err := c.Wallets.List(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(wallets) != 2 {
		t.Errorf("len = %d", len(wallets))
	}
}

func TestWalletService_Remove(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %q", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	err := c.Wallets.Remove(context.Background(), "0xabc")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

// ── APIKeyService ───────────────────────────────────────────────────

func TestAPIKeyService_Create(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/user/api-keys" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIKeyCreateResponse{
			KeyID: "key-123", Name: "my-bot", Prefix: "ak_live_", Key: "ak_live_fullkey",
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.APIKeys.Create(context.Background(), "my-bot")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Key != "ak_live_fullkey" {
		t.Errorf("Key = %q", result.Key)
	}
}

func TestAPIKeyService_List(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": []APIKeyListItem{{KeyID: "k-1"}, {KeyID: "k-2"}},
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	items, err := c.APIKeys.List(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len = %d", len(items))
	}
}

func TestAPIKeyService_Revoke(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/v1/user/api-keys/k-1" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	err := c.APIKeys.Revoke(context.Background(), "k-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

// ── BundleService ───────────────────────────────────────────────────

func TestBundleService_List(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/bundles" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BundleListResponse{
			Catalog:         []BundleCatalogEntry{{Name: "Starter", Pushes: 60, Price: 5}},
			PerRequestPrice: 0.10,
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Bundles.List(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.PerRequestPrice != 0.10 {
		t.Errorf("PerRequestPrice = %f", result.PerRequestPrice)
	}
}

func TestBundleService_Purchase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BundlePurchaseResponse{BundleID: "b-1", TotalPushes: 60})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Bundles.Purchase(context.Background(), PurchaseBundleInput{BundleIndex: 0, WalletAddress: "0x1"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.TotalPushes != 60 {
		t.Errorf("TotalPushes = %d", result.TotalPushes)
	}
}

// ── PushService ─────────────────────────────────────────────────────

func TestPushService_List(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": []PushSubscriptionResponse{{SubscriptionID: "ps-1", Endpoint: "https://push.example.com"}},
		})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	items, err := c.Push.List(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("len = %d", len(items))
	}
}

func TestPushService_Create(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PushSubscriptionResponse{SubscriptionID: "ps-1"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Push.Create(context.Background(), CreatePushInput{
		Endpoint: "https://push.example.com",
		Keys:     PushKeys{P256DH: "key1", Auth: "key2"},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.SubscriptionID != "ps-1" {
		t.Errorf("SubscriptionID = %q", result.SubscriptionID)
	}
}

func TestPushService_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %q", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	err := c.Push.Delete(context.Background(), "ps-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

// ── RedeemService ───────────────────────────────────────────────────

func TestRedeemService_Redeem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/user/redeem" {
			t.Errorf("method=%q path=%q", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["code"] != "PROMO-2026" {
			t.Errorf("code = %q", body["code"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RedeemResponse{Tier: "pro", ExpiresAt: "2026-12-31T00:00:00Z"})
	}))
	defer ts.Close()

	c := mustNewClient(t, WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.Redeem.Redeem(context.Background(), "PROMO-2026")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Tier != "pro" {
		t.Errorf("Tier = %q", result.Tier)
	}
}
