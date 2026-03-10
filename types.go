package attago

import "context"

// ── Constants ───────────────────────────────────────────────────────

// DefaultBaseURL is the AttaGo production API base URL.
const DefaultBaseURL = "https://api.attago.bid"

// DefaultCognitoRegion is the default AWS region for Cognito.
const DefaultCognitoRegion = "us-east-1"

// Version is the SDK version string.
const Version = "0.1.0"

// ── x402 Signer ─────────────────────────────────────────────────────

// X402Signer handles wallet-based x402 payment signing.
// Implementations handle EVM (EIP-712) or Solana (ed25519) signing.
type X402Signer interface {
	// Address returns the wallet address (0x-prefixed EVM or base58 Solana).
	Address() string
	// Network returns the network identifier (e.g. "eip155:8453" for Base).
	Network() string
	// Sign signs an x402 payment payload, returning a base64-encoded payment string.
	Sign(ctx context.Context, requirements *X402PaymentRequirements) (string, error)
}

// X402PaymentRequirements contains decoded x402 payment requirements from
// the PAYMENT-REQUIRED response header.
type X402PaymentRequirements struct {
	X402Version int                  `json:"x402Version"`
	Resource    X402Resource         `json:"resource"`
	Accepts     []X402AcceptedPayment `json:"accepts"`
}

// X402Resource describes the protected resource.
type X402Resource struct {
	URL         string `json:"url"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// X402AcceptedPayment is one accepted payment option from x402 requirements.
type X402AcceptedPayment struct {
	Scheme            string         `json:"scheme"`
	Network           string         `json:"network"`
	Amount            string         `json:"amount"`
	Asset             string         `json:"asset"`
	PayTo             string         `json:"payTo"`
	MaxTimeoutSeconds int            `json:"maxTimeoutSeconds"`
	Extra             map[string]any `json:"extra,omitempty"`
}

// ── Auth types ──────────────────────────────────────────────────────

// CognitoTokens holds the Cognito token set for persistence/restoration.
type CognitoTokens struct {
	IDToken      string `json:"idToken"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// SignUpInput holds the parameters for account registration.
type SignUpInput struct {
	Email           string
	Password        string
	CognitoClientID string
	CognitoRegion   string // defaults to DefaultCognitoRegion if empty
}

// ConfirmSignUpInput holds the parameters for confirming a new account.
type ConfirmSignUpInput struct {
	Email           string
	Code            string
	CognitoClientID string
	CognitoRegion   string
}

// ForgotPasswordInput holds the parameters for triggering a password reset.
type ForgotPasswordInput struct {
	Email           string
	CognitoClientID string
	CognitoRegion   string
}

// ConfirmForgotPasswordInput holds the parameters for completing a password reset.
type ConfirmForgotPasswordInput struct {
	Email           string
	Code            string
	NewPassword     string
	CognitoClientID string
	CognitoRegion   string
}

// ── Agent types ─────────────────────────────────────────────────────

// CompositeScore is the top-level Go/No-Go signal summary.
type CompositeScore struct {
	Score      float64 `json:"score"`
	Signal     string  `json:"signal"`     // "GO", "NO-GO", "NEUTRAL"
	Confidence float64 `json:"confidence"`
}

// AgentScoreResponse is the response from GET /v1/agent/score.
type AgentScoreResponse struct {
	Token          string         `json:"token"`
	Composite      CompositeScore `json:"composite"`
	Spot           map[string]any `json:"spot"`
	Perps          map[string]any `json:"perps"`    // nil if no derivatives
	Context        map[string]any `json:"context"`
	Market         map[string]any `json:"market"`
	DerivSymbols   []string       `json:"derivSymbols"`
	HasDerivatives bool           `json:"hasDerivatives"`
	Sources        []map[string]any `json:"sources"`
	Meta           map[string]any `json:"meta"`
	RequestID      string         `json:"requestId,omitempty"`
}

// AgentDataResponse is the response from GET /v1/agent/data.
type AgentDataResponse struct {
	Assets     map[string]map[string]any `json:"assets"`
	AssetOrder []string                  `json:"assetOrder"`
	Market     map[string]any            `json:"market"`
	Sources    any                       `json:"sources"`
	Meta       map[string]any            `json:"meta"`
	RequestID  string                    `json:"requestId,omitempty"`
}

// ── Data types ──────────────────────────────────────────────────────

// DataLatestResponse is the response from GET /v1/data/latest.
type DataLatestResponse struct {
	Assets     map[string]map[string]any `json:"assets"`
	AssetOrder []string                  `json:"assetOrder"`
	Market     map[string]any            `json:"market"`
	Sources    []map[string]any          `json:"sources"`
	Meta       map[string]any            `json:"meta"`
}

// DataTokenResponse is the response from GET /v1/api/data/{token}.
type DataTokenResponse struct {
	Token          string         `json:"token"`
	Composite      map[string]any `json:"composite"`
	Spot           map[string]any `json:"spot"`
	Perps          map[string]any `json:"perps"`
	Context        map[string]any `json:"context"`
	Market         map[string]any `json:"market"`
	DerivSymbols   []string       `json:"derivSymbols"`
	HasDerivatives bool           `json:"hasDerivatives"`
	Sources        []map[string]any `json:"sources"`
	Meta           map[string]any `json:"meta"`
	RequestID      string         `json:"requestId"`
	Mode           string         `json:"mode"` // "test" or "live"
	Bundle         *BundleUsage   `json:"bundle,omitempty"`
	IncludedPush   *PushUsage     `json:"includedPush,omitempty"`
}

// BundleUsage tracks bundle credit consumption in a data-push response.
type BundleUsage struct {
	BundleID  string `json:"bundleId"`
	Remaining int    `json:"remaining"`
}

// PushUsage tracks included push consumption in a data-push response.
type PushUsage struct {
	Used      int `json:"used"`
	Total     int `json:"total"`
	Remaining int `json:"remaining"`
}

// DataPushResponse is the response from GET /v1/data/push/{requestId}.
type DataPushResponse struct {
	RequestID string         `json:"requestId"`
	TokenID   string         `json:"tokenId"`
	CreatedAt string         `json:"createdAt"`
	Data      map[string]any `json:"data"`
}

// ── Subscription types ──────────────────────────────────────────────

// SubscriptionCondition is a single alert condition (metric + operator + threshold).
type SubscriptionCondition struct {
	MetricName   string `json:"metricName"`
	ThresholdOp  string `json:"thresholdOp"`  // "gt", "lt", "gte", "lte", "eq"
	ThresholdVal any    `json:"thresholdVal"` // number or string
}

// Subscription is an alert subscription returned by the API.
type Subscription struct {
	UserID           string                      `json:"userId"`
	SubID            string                      `json:"subId"`
	TokenID          string                      `json:"tokenId"`
	Label            string                      `json:"label"`
	Groups           [][]SubscriptionCondition   `json:"groups"` // OR-of-ANDs
	CooldownMinutes  int                         `json:"cooldownMinutes"`
	BucketHash       string                      `json:"bucketHash"`
	IsActive         bool                        `json:"isActive"`
	ActiveTokenShard string                      `json:"activeTokenShard,omitempty"`
	CreatedAt        string                      `json:"createdAt"`
	UpdatedAt        string                      `json:"updatedAt"`
}

// CatalogMetric is a metric definition from the subscription catalog.
type CatalogMetric struct {
	Label     string   `json:"label"`
	Type      string   `json:"type"` // "number" or "enum"
	Unit      string   `json:"unit,omitempty"`
	Operators []string `json:"operators"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Values    []string `json:"values,omitempty"`
}

// CatalogResponse is the response from GET /v1/subscriptions/catalog.
type CatalogResponse struct {
	Tokens           []string                  `json:"tokens"`
	Metrics          map[string]CatalogMetric  `json:"metrics"`
	Tier             string                    `json:"tier"`
	MaxSubscriptions int                       `json:"maxSubscriptions"`
	Mode             string                    `json:"mode"`
}

// CreateSubscriptionInput holds the parameters for creating an alert subscription.
type CreateSubscriptionInput struct {
	TokenID         string                    `json:"tokenId"`
	Label           string                    `json:"label"`
	Groups          [][]SubscriptionCondition `json:"groups"`
	CooldownMinutes *int                      `json:"cooldownMinutes,omitempty"` // defaults to 5
}

// UpdateSubscriptionInput holds the parameters for updating a subscription.
// All fields are optional — only non-nil fields are sent.
type UpdateSubscriptionInput struct {
	Label           *string                    `json:"label,omitempty"`
	Groups          *[][]SubscriptionCondition `json:"groups,omitempty"`
	CooldownMinutes *int                       `json:"cooldownMinutes,omitempty"`
	IsActive        *bool                      `json:"isActive,omitempty"`
}

// ── Payment types ───────────────────────────────────────────────────

// SubscribeInput holds the parameters for subscribing to a billing tier.
type SubscribeInput struct {
	Tier         string `json:"tier"`         // "basic", "pro", "business"
	BillingCycle string `json:"billingCycle"` // "monthly", "yearly"
	Renew        bool   `json:"renew"`
}

// SubscribeResponse is the response from a successful subscription.
type SubscribeResponse struct {
	Tier         string  `json:"tier"`
	BillingCycle string  `json:"billingCycle"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
	ExpiresAt    string  `json:"expiresAt"`
	Payer        string  `json:"payer"`
	Mode         string  `json:"mode"`
	Message      string  `json:"message"`
}

// IncludedPushes tracks push usage within a billing period.
type IncludedPushes struct {
	Total       int    `json:"total"`
	Used        int    `json:"used"`
	Remaining   int    `json:"remaining"`
	PeriodStart string `json:"periodStart"`
	PeriodEnd   string `json:"periodEnd"`
}

// BillingStatus is the current billing/tier status.
type BillingStatus struct {
	Tier           string          `json:"tier"`
	TierName       string          `json:"tierName"`
	BillingCycle   string          `json:"billingCycle"`
	ExpiresAt      *string         `json:"expiresAt"` // null for free tier
	MaxSubs        int             `json:"maxSubs"`
	APIAccess      bool            `json:"apiAccess"`
	FreeDataPushes int             `json:"freeDataPushes"`
	Mode           string          `json:"mode"`
	IncludedPushes *IncludedPushes `json:"includedPushes,omitempty"`
}

// UpgradeQuote is a pro-rated upgrade price quote.
type UpgradeQuote struct {
	CurrentTier      string  `json:"currentTier"`
	CurrentCycle     string  `json:"currentCycle"`
	CurrentExpiresAt string  `json:"currentExpiresAt"`
	TargetTier       string  `json:"targetTier"`
	TargetCycle      string  `json:"targetCycle"`
	BasePrice        float64 `json:"basePrice"`
	ProrationCredit  float64 `json:"prorationCredit"`
	FinalPrice       float64 `json:"finalPrice"`
	Currency         string  `json:"currency"`
	ExpiresAt        string  `json:"expiresAt"`
}

// ── Wallet types ────────────────────────────────────────────────────

// RegisterWalletInput holds the parameters for registering a wallet.
type RegisterWalletInput struct {
	WalletAddress string `json:"walletAddress"`
	Chain         string `json:"chain"` // "base", "avax", "polygon", "arbitrum", "optimism", "solana"
	Signature     string `json:"signature"`
	Timestamp     int64  `json:"timestamp"` // Unix seconds
}

// Wallet is a verified wallet returned by the API.
type Wallet struct {
	UserID        string `json:"userId"`
	WalletAddress string `json:"walletAddress"`
	Chain         string `json:"chain"`
	VerifiedAt    string `json:"verifiedAt"`
}

// ── Webhook types ───────────────────────────────────────────────────

// WebhookCreateResponse is returned on webhook creation (includes secret — shown only once).
type WebhookCreateResponse struct {
	WebhookID string `json:"webhookId"`
	URL       string `json:"url"`
	Secret    string `json:"secret"`
	CreatedAt string `json:"createdAt"`
}

// WebhookListItem is a webhook in a list response (secret stripped).
type WebhookListItem struct {
	WebhookID string `json:"webhookId"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
}

// WebhookTestResult is the result of a test delivery (SDK-side or server-side).
type WebhookTestResult struct {
	Success    bool   `json:"success"`
	StatusCode *int   `json:"statusCode"` // nil on network error
	Attempts   int    `json:"attempts"`
	Error      string `json:"error,omitempty"`
}

// SendTestOptions configures SDK-side webhook test delivery.
type SendTestOptions struct {
	// URL is the webhook endpoint URL.
	URL string
	// Secret is the HMAC signing secret (from webhook creation response).
	Secret string
	// Token is the token symbol for the test payload. Defaults to "BTC".
	Token string
	// State is the alert state. Defaults to "triggered".
	State string
	// Environment is the environment label. Defaults to "production".
	Environment string
	// BackoffMs overrides retry backoff delays (for testing). Default: [1000, 4000, 16000].
	BackoffMs []int
}

// WebhookPayload is a v2 webhook delivery payload (alert or test).
type WebhookPayload struct {
	Event       string              `json:"event"` // "alert" or "test"
	Version     string              `json:"version"`
	Environment string              `json:"environment"`
	Alert       WebhookPayloadAlert `json:"alert"`
	Data        WebhookPayloadData  `json:"data"`
	Timestamp   string              `json:"timestamp"`
}

// WebhookPayloadAlert is the alert section of a webhook payload.
type WebhookPayloadAlert struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Token string `json:"token"`
	State string `json:"state"` // "triggered" or "resolved"
}

// WebhookPayloadData is the data section of a webhook payload.
type WebhookPayloadData struct {
	URL         *string `json:"url"`
	ExpiresAt   string  `json:"expiresAt,omitempty"`
	FallbackURL string  `json:"fallbackUrl,omitempty"`
}

// ── API Key types ───────────────────────────────────────────────────

// APIKeyCreateResponse is returned on API key creation (includes raw key — shown only once).
type APIKeyCreateResponse struct {
	KeyID     string `json:"keyId"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Key       string `json:"key"` // full raw key, shown only once
	CreatedAt string `json:"createdAt"`
}

// APIKeyListItem is an API key in a list response (raw key never shown).
type APIKeyListItem struct {
	KeyID      string  `json:"keyId"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
	RevokedAt  *string `json:"revokedAt"`
}

// ── Bundle types ────────────────────────────────────────────────────

// Bundle is a purchased data-push credit bundle.
type Bundle struct {
	BundleID      string  `json:"bundleId"`
	UserID        string  `json:"userId"`
	WalletAddress string  `json:"walletAddress"`
	BundleSize    int     `json:"bundleSize"`
	Remaining     int     `json:"remaining"`
	PurchasedAt   string  `json:"purchasedAt"`
	ExpiresAt     *string `json:"expiresAt"`
}

// BundleCatalogEntry is an available bundle package.
type BundleCatalogEntry struct {
	Name   string  `json:"name"`
	Pushes int     `json:"pushes"`
	Price  float64 `json:"price"`
}

// BundleListResponse is the response from listing bundles.
type BundleListResponse struct {
	Bundles         []Bundle             `json:"bundles"`
	Catalog         []BundleCatalogEntry `json:"catalog"`
	PerRequestPrice float64              `json:"perRequestPrice"`
}

// PurchaseBundleInput holds the parameters for purchasing a bundle.
type PurchaseBundleInput struct {
	BundleIndex   int    `json:"bundleIndex"`
	WalletAddress string `json:"walletAddress"`
}

// BundlePurchaseResponse is the response from a successful bundle purchase.
type BundlePurchaseResponse struct {
	BundleID      string  `json:"bundleId"`
	UserID        string  `json:"userId"`
	WalletAddress string  `json:"walletAddress"`
	BundleName    string  `json:"bundleName"`
	TotalPushes   int     `json:"totalPushes"`
	Remaining     int     `json:"remaining"`
	Price         float64 `json:"price"`
	PurchasedAt   string  `json:"purchasedAt"`
	Payer         string  `json:"payer"`
	TransactionID string  `json:"transactionId"`
}

// ── Push types ──────────────────────────────────────────────────────

// PushKeys holds the Web Push subscription encryption keys.
type PushKeys struct {
	P256DH string `json:"p256dh"`
	Auth   string `json:"auth"`
}

// CreatePushInput holds the parameters for registering a push subscription.
type CreatePushInput struct {
	Endpoint string   `json:"endpoint"`
	Keys     PushKeys `json:"keys"`
}

// PushSubscriptionResponse is a push subscription returned by the API.
type PushSubscriptionResponse struct {
	SubscriptionID string `json:"subscriptionId"`
	Endpoint       string `json:"endpoint"`
	CreatedAt      string `json:"createdAt"`
}

// ── Redeem types ────────────────────────────────────────────────────

// RedeemResponse is the response from a successful code redemption.
type RedeemResponse struct {
	Tier      string `json:"tier"`
	ExpiresAt string `json:"expiresAt"`
	Message   string `json:"message"`
}

// ── MCP types ───────────────────────────────────────────────────────

// MCPServerInfo is the response from MCP initialize.
type MCPServerInfo struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    MCPCapabilities   `json:"capabilities"`
	ServerInfo      MCPServerMetadata `json:"serverInfo"`
	Instructions    string            `json:"instructions,omitempty"`
}

// MCPCapabilities describes server capabilities.
type MCPCapabilities struct {
	Tools *MCPToolsCapability `json:"tools,omitempty"`
}

// MCPToolsCapability describes the tools capability.
type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// MCPServerMetadata is the server identity info.
type MCPServerMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPTool is a tool definition from tools/list.
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// MCPToolContent is a content item in a tool result.
type MCPToolContent struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// MCPToolCallResult is the result of calling an MCP tool.
type MCPToolCallResult struct {
	Content []MCPToolContent `json:"content"`
	IsError bool             `json:"isError"`
}

// ── User profile types ──────────────────────────────────────────────

// UserProfile is the response from GET /v1/user/profile.
type UserProfile struct {
	UserID             string  `json:"userId"`
	Email              string  `json:"email"`
	PlanTier           string  `json:"planTier"`
	Role               string  `json:"role"`
	EffectiveTier      string  `json:"effectiveTier"`
	TierOverride       *string `json:"tierOverride"`
	ArenaUsername      *string `json:"arenaUsername"`
	DeliveryPreference string  `json:"deliveryPreference"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}
