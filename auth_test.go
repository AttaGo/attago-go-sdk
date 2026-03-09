package attago

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── CognitoAuth.SignIn ──────────────────────────────────────────────

func TestCognitoAuth_SignIn_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		if target != "AWSCognitoIdentityProviderService.InitiateAuth" {
			t.Errorf("X-Amz-Target = %q", target)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-amz-json-1.1" {
			t.Errorf("Content-Type = %q", ct)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"AuthenticationResult": map[string]any{
				"IdToken":      "id-token-abc",
				"AccessToken":  "access-token-xyz",
				"RefreshToken": "refresh-token-123",
			},
		})
	}))
	defer ts.Close()

	// Override region to point at test server
	auth := newCognitoAuth("client-id", "us-east-1", http.DefaultClient, "user@example.com", "password123")
	// Replace the endpoint by overriding the http client
	auth.httpClient = &http.Client{
		Transport: &rewriteTransport{base: http.DefaultTransport, target: ts.URL},
	}

	err := auth.SignIn(context.Background())
	if err != nil {
		t.Fatalf("SignIn() error: %v", err)
	}
	tokens := auth.GetTokens()
	if tokens == nil {
		t.Fatal("GetTokens() = nil after sign-in")
	}
	if tokens.IDToken != "id-token-abc" {
		t.Errorf("IDToken = %q", tokens.IDToken)
	}
}

func TestCognitoAuth_SignIn_MFAChallenge(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ChallengeName": "SOFTWARE_TOKEN_MFA",
			"Session":       "session-token-xyz",
		})
	}))
	defer ts.Close()

	auth := newCognitoAuth("client-id", "us-east-1",
		&http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: ts.URL}},
		"user@example.com", "pass")

	err := auth.SignIn(context.Background())
	if err == nil {
		t.Fatal("expected MFARequiredError")
	}

	var mfaErr *MFARequiredError
	if !errors.As(err, &mfaErr) {
		t.Fatalf("expected *MFARequiredError, got %T: %v", err, err)
	}
	if mfaErr.Session != "session-token-xyz" {
		t.Errorf("Session = %q", mfaErr.Session)
	}
	if mfaErr.ChallengeName != "SOFTWARE_TOKEN_MFA" {
		t.Errorf("ChallengeName = %q", mfaErr.ChallengeName)
	}
}

func TestCognitoAuth_SignIn_AuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{
			"__type":  "NotAuthorizedException",
			"message": "Incorrect username or password",
		})
	}))
	defer ts.Close()

	auth := newCognitoAuth("client-id", "us-east-1",
		&http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: ts.URL}},
		"user@example.com", "wrong")

	err := auth.SignIn(context.Background())
	if err == nil {
		t.Fatal("expected AuthError")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if authErr.Code != "NotAuthorizedException" {
		t.Errorf("Code = %q", authErr.Code)
	}
}

// ── Token management ────────────────────────────────────────────────

func TestCognitoAuth_SetTokens_GetIDToken(t *testing.T) {
	auth := newCognitoAuth("client-id", "us-east-1", http.DefaultClient, "", "")
	auth.SetTokens(&CognitoTokens{
		IDToken:      "preset-id-token",
		AccessToken:  "preset-access",
		RefreshToken: "preset-refresh",
	})

	token, err := auth.GetIDToken(context.Background())
	if err != nil {
		t.Fatalf("GetIDToken() error: %v", err)
	}
	if token != "preset-id-token" {
		t.Errorf("IDToken = %q", token)
	}
}

func TestCognitoAuth_GetIDToken_NoTokens(t *testing.T) {
	auth := newCognitoAuth("client-id", "us-east-1", http.DefaultClient, "", "")
	_, err := auth.GetIDToken(context.Background())
	if err == nil {
		t.Fatal("expected error when no tokens available")
	}
}

func TestCognitoAuth_SignOut_ClearsTokens(t *testing.T) {
	auth := newCognitoAuth("client-id", "us-east-1", http.DefaultClient, "", "")
	auth.SetTokens(&CognitoTokens{IDToken: "x"})
	auth.SignOut()
	if auth.GetTokens() != nil {
		t.Error("tokens should be nil after sign-out")
	}
}

// ── RespondToMFA ────────────────────────────────────────────────────

func TestCognitoAuth_RespondToMFA_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		if target != "AWSCognitoIdentityProviderService.RespondToAuthChallenge" {
			t.Errorf("X-Amz-Target = %q", target)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"AuthenticationResult": map[string]any{
				"IdToken":      "mfa-id-token",
				"AccessToken":  "mfa-access-token",
				"RefreshToken": "mfa-refresh-token",
			},
		})
	}))
	defer ts.Close()

	auth := newCognitoAuth("client-id", "us-east-1",
		&http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: ts.URL}},
		"", "")

	err := auth.RespondToMFA(context.Background(), "session-123", "123456")
	if err != nil {
		t.Fatalf("RespondToMFA() error: %v", err)
	}
	if auth.GetTokens().IDToken != "mfa-id-token" {
		t.Errorf("IDToken = %q", auth.GetTokens().IDToken)
	}
}

// ── Helper: rewrite transport to redirect Cognito requests to test server ──

type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.target[len("http://"):]
	return t.base.RoundTrip(req)
}
