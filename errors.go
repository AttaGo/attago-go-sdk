// Package attago provides a Go SDK for the AttaGo crypto trading dashboard API.
//
// Three authentication modes:
//   - API key:     NewClient(WithAPIKey("ak_live_..."))
//   - x402 signer: NewClient(WithSigner(walletSigner))
//   - Cognito:     NewClient(WithCognito(email, password, clientID))
package attago

import "fmt"

// ── Error hierarchy ─────────────────────────────────────────────────
//
//	*APIError               (HTTP 4xx/5xx)
//	  ├── *PaymentRequiredError   (402)
//	  └── *RateLimitError         (429)
//	*AuthError              (Cognito)
//	  └── *MFARequiredError
//	*MCPError               (JSON-RPC 2.0)
//
// All error types implement the error interface and support errors.As().

// APIError represents an HTTP 4xx/5xx error from the AttaGo API.
type APIError struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Message is the error message from the API response body.
	Message string
	// Body is the parsed response body (may be nil).
	Body map[string]any
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("attago: HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("attago: HTTP %d", e.StatusCode)
}

// PaymentRequiredError is a 402 Payment Required response with x402 payment
// requirements. The PaymentRequirements field contains the decoded
// PAYMENT-REQUIRED header with accepted networks, amounts, and signing info.
type PaymentRequiredError struct {
	*APIError
	// PaymentRequirements contains the decoded x402 payment requirements.
	PaymentRequirements *X402PaymentRequirements
}

func (e *PaymentRequiredError) Error() string {
	return fmt.Sprintf("attago: payment required: %s", e.Message)
}

func (e *PaymentRequiredError) Unwrap() error { return e.APIError }

// RateLimitError is a 429 Too Many Requests response, optionally with a
// Retry-After hint.
type RateLimitError struct {
	*APIError
	// RetryAfter is seconds until the ban expires (from Retry-After header).
	// Zero if the header was not present.
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("attago: rate limited (retry after %ds): %s", e.RetryAfter, e.Message)
	}
	return fmt.Sprintf("attago: rate limited: %s", e.Message)
}

func (e *RateLimitError) Unwrap() error { return e.APIError }

// AuthError represents a Cognito authentication error.
type AuthError struct {
	// Message is the human-readable error message.
	Message string
	// Code is the Cognito error type (e.g. "NotAuthorizedException").
	Code string
}

func (e *AuthError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("attago: auth error [%s]: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("attago: auth error: %s", e.Message)
}

// MFARequiredError indicates that MFA is required to complete sign-in.
// Call CognitoAuth.RespondToMFA with Session and the TOTP code.
type MFARequiredError struct {
	*AuthError
	// Session is the Cognito session token — pass to RespondToMFA().
	Session string
	// ChallengeName is the challenge type (e.g. "SOFTWARE_TOKEN_MFA").
	ChallengeName string
}

func (e *MFARequiredError) Error() string {
	return fmt.Sprintf("attago: MFA required (%s)", e.ChallengeName)
}

func (e *MFARequiredError) Unwrap() error { return e.AuthError }

// MCPError represents a JSON-RPC 2.0 error from the MCP server.
type MCPError struct {
	// Code is the JSON-RPC error code (e.g. -32601 for method not found).
	Code int
	// Message is the error message.
	Message string
	// Data is optional additional error data.
	Data any
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("attago: MCP error %d: %s", e.Code, e.Message)
}
