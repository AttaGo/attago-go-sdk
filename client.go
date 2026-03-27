package attago

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ── Functional options ──────────────────────────────────────────────

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets API key authentication (e.g. "ak_live_abc123...").
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithSigner sets x402 wallet signer for payment-authenticated requests.
func WithSigner(s X402Signer) Option {
	return func(c *Client) { c.signer = s }
}

// WithCognito sets Cognito email/password authentication.
// The CognitoAuth instance is created lazily on the Client.
func WithCognito(email, password, clientID string) Option {
	return func(c *Client) {
		c.cognitoEmail = email
		c.cognitoPassword = password
		c.cognitoClientID = clientID
	}
}

// WithCognitoRegion overrides the default Cognito region (us-east-1).
func WithCognitoRegion(region string) Option {
	return func(c *Client) { c.cognitoRegion = region }
}

// WithBaseURL overrides the default API base URL.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient sets a custom HTTP client (for timeouts, TLS, proxies).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// ── Auth mode ───────────────────────────────────────────────────────

// AuthMode describes which authentication method is active.
type AuthMode string

const (
	AuthModeAPIKey  AuthMode = "apikey"
	AuthModeX402    AuthMode = "x402"
	AuthModeCognito AuthMode = "cognito"
	AuthModeNone    AuthMode = "none"
)

// ── Client ──────────────────────────────────────────────────────────

// Client is the main entry point for the AttaGo SDK.
// Use NewClient with functional options to create one.
type Client struct {
	// BaseURL is the resolved API base URL (no trailing slash).
	baseURL    string
	httpClient *http.Client

	// Auth fields
	apiKey          string
	signer          X402Signer
	cognitoEmail    string
	cognitoPassword string
	cognitoClientID string
	cognitoRegion   string

	// Auth is the Cognito auth manager (nil unless cognito mode).
	Auth *CognitoAuth

	// ── Service namespaces ──
	Agent         *AgentService
	APIKeys       *APIKeyService
	Bundles       *BundleService
	Data          *DataService
	MCP           *MCPService
	Messaging     *MessagingService
	Payments      *PaymentService
	Push          *PushService
	Redeem        *RedeemService
	Subscriptions *SubscriptionService
	Wallets       *WalletService
	Webhooks      *WebhookService
}

// NewClient creates a new AttaGo API client.
//
// Only one auth mode is allowed at a time:
//
//	attago.NewClient(attago.WithAPIKey("ak_live_..."))
//	attago.NewClient(attago.WithSigner(walletSigner))
//	attago.NewClient(attago.WithCognito(email, password, clientID))
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:       DefaultBaseURL,
		httpClient:    http.DefaultClient,
		cognitoRegion: DefaultCognitoRegion,
	}

	for _, opt := range opts {
		opt(c)
	}

	// ── Validate: only one auth mode ──
	modes := 0
	if c.apiKey != "" {
		modes++
	}
	if c.signer != nil {
		modes++
	}
	if c.cognitoEmail != "" || c.cognitoClientID != "" {
		modes++
	}
	if modes > 1 {
		return nil, fmt.Errorf("attago: only one auth mode allowed (apiKey, signer, or cognito)")
	}

	// ── Set up Cognito auth ──
	if c.AuthMode() == AuthModeCognito {
		if c.cognitoClientID == "" {
			return nil, fmt.Errorf("attago: cognitoClientID is required for Cognito authentication")
		}
		c.Auth = newCognitoAuth(c.cognitoClientID, c.cognitoRegion, c.httpClient, c.cognitoEmail, c.cognitoPassword)
	}

	// ── Attach service namespaces ──
	c.Agent = &AgentService{client: c}
	c.APIKeys = &APIKeyService{client: c}
	c.Bundles = &BundleService{client: c}
	c.Data = &DataService{client: c}
	c.MCP = &MCPService{client: c}
	c.Messaging = &MessagingService{client: c}
	c.Payments = &PaymentService{client: c}
	c.Push = &PushService{client: c}
	c.Redeem = &RedeemService{client: c}
	c.Subscriptions = &SubscriptionService{client: c}
	c.Wallets = &WalletService{client: c}
	c.Webhooks = &WebhookService{client: c}

	return c, nil
}

// AuthMode returns the active authentication mode.
func (c *Client) AuthMode() AuthMode {
	switch {
	case c.apiKey != "":
		return AuthModeAPIKey
	case c.signer != nil:
		return AuthModeX402
	case c.cognitoEmail != "" || c.cognitoClientID != "":
		return AuthModeCognito
	default:
		return AuthModeNone
	}
}

// Signer returns the configured x402 signer, or nil.
func (c *Client) Signer() X402Signer { return c.signer }

// ── Request options ─────────────────────────────────────────────────

// RequestOption configures a single API request.
type RequestOption func(*requestConfig)

type requestConfig struct {
	body    any
	headers map[string]string
	query   url.Values
}

// WithBody sets the JSON request body.
func WithBody(body any) RequestOption {
	return func(rc *requestConfig) { rc.body = body }
}

// WithHeader adds a header to the request.
func WithHeader(key, value string) RequestOption {
	return func(rc *requestConfig) {
		if rc.headers == nil {
			rc.headers = make(map[string]string)
		}
		rc.headers[key] = value
	}
}

// WithQuery adds a query parameter.
func WithQuery(key, value string) RequestOption {
	return func(rc *requestConfig) {
		if rc.query == nil {
			rc.query = make(url.Values)
		}
		rc.query.Set(key, value)
	}
}

// ── Internal request infrastructure ─────────────────────────────────

// do executes an authenticated HTTP request and decodes the JSON response.
// The /v1 prefix is added automatically if not present.
func (c *Client) do(ctx context.Context, method, path string, result any, opts ...RequestOption) error {
	rc := &requestConfig{}
	for _, opt := range opts {
		opt(rc)
	}

	// ── Build URL ──
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasPrefix(path, "/v1/") {
		path = "/v1" + path
	}

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("attago: invalid URL: %w", err)
	}
	if rc.query != nil {
		u.RawQuery = rc.query.Encode()
	}

	// ── Build body ──
	var bodyReader io.Reader
	var bodyBytes []byte
	if rc.body != nil {
		bodyBytes, err = json.Marshal(rc.body)
		if err != nil {
			return fmt.Errorf("attago: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("attago: build request: %w", err)
	}

	// ── Headers ──
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "attago-go-sdk/"+Version)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range rc.headers {
		req.Header.Set(k, v)
	}

	// ── Auth ──
	switch c.AuthMode() {
	case AuthModeAPIKey:
		req.Header.Set("X-API-Key", c.apiKey)
	case AuthModeCognito:
		if c.Auth != nil {
			token, tokenErr := c.Auth.GetIDToken(ctx)
			if tokenErr != nil {
				return tokenErr
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	// x402: no auth header — payment signature added by doWithX402

	// ── Execute ──
	var res *http.Response
	if c.AuthMode() == AuthModeX402 && c.signer != nil {
		res, err = doWithX402(ctx, c.httpClient, c.signer, req, bodyBytes)
	} else {
		res, err = c.httpClient.Do(req)
	}
	if err != nil {
		return fmt.Errorf("attago: request failed: %w", err)
	}
	defer res.Body.Close()

	// ── Handle errors ──
	if res.StatusCode >= 400 {
		return handleHTTPError(res)
	}

	// ── Parse response ──
	if res.StatusCode == 204 || result == nil {
		return nil
	}

	if err := json.NewDecoder(io.LimitReader(res.Body, 10<<20)).Decode(result); err != nil {
		return fmt.Errorf("attago: decode response: %w", err)
	}
	return nil
}

// handleHTTPError maps HTTP error responses to typed errors.
func handleHTTPError(res *http.Response) error {
	bodyBytes, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))

	var body map[string]any
	_ = json.Unmarshal(bodyBytes, &body)

	message := ""
	if s, ok := body["error"].(string); ok {
		message = s
	} else if s, ok := body["message"].(string); ok {
		message = s
	} else {
		message = fmt.Sprintf("HTTP %d", res.StatusCode)
	}

	apiErr := &APIError{
		StatusCode: res.StatusCode,
		Message:    message,
		Body:       body,
	}

	switch res.StatusCode {
	case 402:
		reqs := ParsePaymentRequired(res.Header)
		return &PaymentRequiredError{
			APIError:            apiErr,
			PaymentRequirements: reqs,
		}
	case 429:
		retryAfter := 0
		if s := res.Header.Get("Retry-After"); s != "" {
			retryAfter, _ = strconv.Atoi(s)
		}
		return &RateLimitError{
			APIError:   apiErr,
			RetryAfter: retryAfter,
		}
	default:
		return apiErr
	}
}
