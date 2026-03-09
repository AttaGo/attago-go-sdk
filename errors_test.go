package attago

import (
	"errors"
	"testing"
)

// ── APIError ────────────────────────────────────────────────────────

func TestAPIError_Error(t *testing.T) {
	e := &APIError{StatusCode: 404, Message: "Not found"}
	got := e.Error()
	want := "attago: HTTP 404: Not found"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_Error_NoMessage(t *testing.T) {
	e := &APIError{StatusCode: 500}
	got := e.Error()
	want := "attago: HTTP 500"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_ErrorsAs(t *testing.T) {
	var apiErr *APIError
	err := error(&APIError{StatusCode: 403, Message: "Forbidden"})
	if !errors.As(err, &apiErr) {
		t.Fatal("errors.As failed for *APIError")
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}

// ── PaymentRequiredError ────────────────────────────────────────────

func TestPaymentRequiredError_Error(t *testing.T) {
	e := &PaymentRequiredError{
		APIError:            &APIError{StatusCode: 402, Message: "Payment required"},
		PaymentRequirements: &X402PaymentRequirements{X402Version: 1},
	}
	got := e.Error()
	want := "attago: payment required: Payment required"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestPaymentRequiredError_Unwrap(t *testing.T) {
	inner := &APIError{StatusCode: 402, Message: "Payment required"}
	e := &PaymentRequiredError{APIError: inner}

	// errors.As should match both PaymentRequiredError and APIError
	var apiErr *APIError
	if !errors.As(e, &apiErr) {
		t.Fatal("errors.As for *APIError should succeed via Unwrap")
	}
	if apiErr.StatusCode != 402 {
		t.Errorf("StatusCode = %d, want 402", apiErr.StatusCode)
	}
}

func TestPaymentRequiredError_ErrorsAs(t *testing.T) {
	err := error(&PaymentRequiredError{
		APIError:            &APIError{StatusCode: 402, Message: "pay up"},
		PaymentRequirements: nil,
	})
	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatal("errors.As failed for *PaymentRequiredError")
	}
}

// ── RateLimitError ──────────────────────────────────────────────────

func TestRateLimitError_Error_WithRetryAfter(t *testing.T) {
	e := &RateLimitError{
		APIError:   &APIError{StatusCode: 429, Message: "Too many requests"},
		RetryAfter: 60,
	}
	got := e.Error()
	want := "attago: rate limited (retry after 60s): Too many requests"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRateLimitError_Error_NoRetryAfter(t *testing.T) {
	e := &RateLimitError{
		APIError: &APIError{StatusCode: 429, Message: "Slow down"},
	}
	got := e.Error()
	want := "attago: rate limited: Slow down"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRateLimitError_Unwrap(t *testing.T) {
	inner := &APIError{StatusCode: 429, Message: "rate limited"}
	e := &RateLimitError{APIError: inner, RetryAfter: 30}

	var apiErr *APIError
	if !errors.As(e, &apiErr) {
		t.Fatal("errors.As for *APIError should succeed via Unwrap")
	}
	if apiErr.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", apiErr.StatusCode)
	}
}

// ── AuthError ───────────────────────────────────────────────────────

func TestAuthError_Error_WithCode(t *testing.T) {
	e := &AuthError{Message: "Bad credentials", Code: "NotAuthorizedException"}
	got := e.Error()
	want := "attago: auth error [NotAuthorizedException]: Bad credentials"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAuthError_Error_NoCode(t *testing.T) {
	e := &AuthError{Message: "Something went wrong"}
	got := e.Error()
	want := "attago: auth error: Something went wrong"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAuthError_ErrorsAs(t *testing.T) {
	err := error(&AuthError{Message: "denied", Code: "AccessDenied"})
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatal("errors.As failed for *AuthError")
	}
	if authErr.Code != "AccessDenied" {
		t.Errorf("Code = %q, want %q", authErr.Code, "AccessDenied")
	}
}

// ── MFARequiredError ────────────────────────────────────────────────

func TestMFARequiredError_Error(t *testing.T) {
	e := &MFARequiredError{
		AuthError:     &AuthError{Message: "MFA required"},
		Session:       "session-token-123",
		ChallengeName: "SOFTWARE_TOKEN_MFA",
	}
	got := e.Error()
	want := "attago: MFA required (SOFTWARE_TOKEN_MFA)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestMFARequiredError_Unwrap(t *testing.T) {
	inner := &AuthError{Message: "MFA required", Code: "MFAChallenge"}
	e := &MFARequiredError{
		AuthError:     inner,
		Session:       "sess-abc",
		ChallengeName: "SOFTWARE_TOKEN_MFA",
	}

	var authErr *AuthError
	if !errors.As(e, &authErr) {
		t.Fatal("errors.As for *AuthError should succeed via Unwrap")
	}
	if authErr.Code != "MFAChallenge" {
		t.Errorf("Code = %q, want %q", authErr.Code, "MFAChallenge")
	}
}

func TestMFARequiredError_ErrorsAs(t *testing.T) {
	err := error(&MFARequiredError{
		AuthError:     &AuthError{Message: "mfa"},
		Session:       "s",
		ChallengeName: "SOFTWARE_TOKEN_MFA",
	})
	var mfaErr *MFARequiredError
	if !errors.As(err, &mfaErr) {
		t.Fatal("errors.As failed for *MFARequiredError")
	}
	if mfaErr.Session != "s" {
		t.Errorf("Session = %q, want %q", mfaErr.Session, "s")
	}
}

// ── MCPError ────────────────────────────────────────────────────────

func TestMCPError_Error(t *testing.T) {
	e := &MCPError{Code: -32601, Message: "Method not found"}
	got := e.Error()
	want := "attago: MCP error -32601: Method not found"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestMCPError_ErrorsAs(t *testing.T) {
	err := error(&MCPError{Code: -32700, Message: "Parse error"})
	var mcpErr *MCPError
	if !errors.As(err, &mcpErr) {
		t.Fatal("errors.As failed for *MCPError")
	}
	if mcpErr.Code != -32700 {
		t.Errorf("Code = %d, want %d", mcpErr.Code, -32700)
	}
}

// ── Cross-hierarchy: PaymentRequired should NOT match AuthError ─────

func TestPaymentRequired_DoesNotMatchAuthError(t *testing.T) {
	err := error(&PaymentRequiredError{
		APIError: &APIError{StatusCode: 402, Message: "pay"},
	})
	var authErr *AuthError
	if errors.As(err, &authErr) {
		t.Error("PaymentRequiredError should not match *AuthError")
	}
}

func TestMCPError_DoesNotMatchAPIError(t *testing.T) {
	err := error(&MCPError{Code: -32601, Message: "not found"})
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Error("MCPError should not match *APIError")
	}
}
