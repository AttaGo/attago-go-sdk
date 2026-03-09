package attago

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// CognitoAuth handles Cognito JWT authentication.
// Use the Client.Auth field to access this in cognito mode.
type CognitoAuth struct {
	clientID   string
	region     string
	httpClient *http.Client
	email      string
	password   string

	mu     sync.RWMutex
	tokens *CognitoTokens
}

// newCognitoAuth creates a CognitoAuth instance. Called internally by NewClient.
func newCognitoAuth(clientID, region string, hc *http.Client, email, password string) *CognitoAuth {
	return &CognitoAuth{
		clientID:   clientID,
		region:     region,
		httpClient: hc,
		email:      email,
		password:   password,
	}
}

// GetIDToken returns a valid ID token, auto-refreshing if expired.
// Performs sign-in on first call if email/password were provided.
func (a *CognitoAuth) GetIDToken(ctx context.Context) (string, error) {
	a.mu.RLock()
	if a.tokens != nil && a.tokens.IDToken != "" {
		// TODO: check expiry and refresh
		tok := a.tokens.IDToken
		a.mu.RUnlock()
		return tok, nil
	}
	a.mu.RUnlock()

	if a.email != "" && a.password != "" {
		if err := a.SignIn(ctx); err != nil {
			return "", err
		}
		a.mu.RLock()
		tok := a.tokens.IDToken
		a.mu.RUnlock()
		return tok, nil
	}

	return "", &AuthError{Message: "No Cognito tokens available — call SignIn() first"}
}

// SignIn authenticates with email/password and stores the token set.
func (a *CognitoAuth) SignIn(ctx context.Context) error {
	tokens, err := cognitoInitiateAuth(ctx, a.httpClient, a.clientID, a.region, a.email, a.password)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.tokens = tokens
	a.mu.Unlock()
	return nil
}

// SignOut clears the stored tokens.
func (a *CognitoAuth) SignOut() {
	a.mu.Lock()
	a.tokens = nil
	a.mu.Unlock()
}

// GetTokens returns the current token set (for persistence).
func (a *CognitoAuth) GetTokens() *CognitoTokens {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tokens
}

// SetTokens restores a previously persisted token set.
func (a *CognitoAuth) SetTokens(tokens *CognitoTokens) {
	a.mu.Lock()
	a.tokens = tokens
	a.mu.Unlock()
}

// RespondToMFA completes an MFA challenge.
func (a *CognitoAuth) RespondToMFA(ctx context.Context, session, code string) error {
	tokens, err := cognitoRespondToMFA(ctx, a.httpClient, a.clientID, a.region, session, code)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.tokens = tokens
	a.mu.Unlock()
	return nil
}

// ── Package-level registration helpers ──────────────────────────────

// SignUp creates a new account. Sends a verification code to the email.
func SignUp(ctx context.Context, input SignUpInput) (string, error) {
	region := input.CognitoRegion
	if region == "" {
		region = DefaultCognitoRegion
	}
	return cognitoSignUp(ctx, http.DefaultClient, input.CognitoClientID, region, input.Email, input.Password)
}

// ConfirmSignUp confirms a new account with the emailed verification code.
func ConfirmSignUp(ctx context.Context, input ConfirmSignUpInput) error {
	region := input.CognitoRegion
	if region == "" {
		region = DefaultCognitoRegion
	}
	return cognitoConfirmSignUp(ctx, http.DefaultClient, input.CognitoClientID, region, input.Email, input.Code)
}

// ForgotPassword triggers a password-reset email.
func ForgotPassword(ctx context.Context, input ForgotPasswordInput) error {
	region := input.CognitoRegion
	if region == "" {
		region = DefaultCognitoRegion
	}
	return cognitoForgotPassword(ctx, http.DefaultClient, input.CognitoClientID, region, input.Email)
}

// ConfirmForgotPassword completes a password reset with the emailed code.
func ConfirmForgotPassword(ctx context.Context, input ConfirmForgotPasswordInput) error {
	region := input.CognitoRegion
	if region == "" {
		region = DefaultCognitoRegion
	}
	return cognitoConfirmForgotPassword(ctx, http.DefaultClient, input.CognitoClientID, region, input.Email, input.Code, input.NewPassword)
}

// ── Cognito REST helpers ────────────────────────────────────────────
// These call the Cognito Identity Provider REST API directly (no SDK).

func cognitoInitiateAuth(ctx context.Context, hc *http.Client, clientID, region, email, password string) (*CognitoTokens, error) {
	body := map[string]any{
		"AuthFlow": "USER_PASSWORD_AUTH",
		"ClientId": clientID,
		"AuthParameters": map[string]string{
			"USERNAME": email,
			"PASSWORD": password,
		},
	}
	result, err := cognitoRequest(ctx, hc, region, "AWSCognitoIdentityProviderService.InitiateAuth", body)
	if err != nil {
		return nil, err
	}

	// Check for MFA challenge
	if challenge, ok := result["ChallengeName"].(string); ok {
		session, _ := result["Session"].(string)
		return nil, &MFARequiredError{
			AuthError:     &AuthError{Message: "MFA required", Code: challenge},
			Session:       session,
			ChallengeName: challenge,
		}
	}

	return extractTokens(result)
}

func cognitoRespondToMFA(ctx context.Context, hc *http.Client, clientID, region, session, code string) (*CognitoTokens, error) {
	body := map[string]any{
		"ChallengeName": "SOFTWARE_TOKEN_MFA",
		"ClientId":      clientID,
		"Session":       session,
		"ChallengeResponses": map[string]string{
			"SOFTWARE_TOKEN_MFA_CODE": code,
		},
	}
	result, err := cognitoRequest(ctx, hc, region, "AWSCognitoIdentityProviderService.RespondToAuthChallenge", body)
	if err != nil {
		return nil, err
	}
	return extractTokens(result)
}

func cognitoSignUp(ctx context.Context, hc *http.Client, clientID, region, email, password string) (string, error) {
	body := map[string]any{
		"ClientId": clientID,
		"Username": email,
		"Password": password,
		"UserAttributes": []map[string]string{
			{"Name": "email", "Value": email},
		},
	}
	result, err := cognitoRequest(ctx, hc, region, "AWSCognitoIdentityProviderService.SignUp", body)
	if err != nil {
		return "", err
	}
	userSub, _ := result["UserSub"].(string)
	return userSub, nil
}

func cognitoConfirmSignUp(ctx context.Context, hc *http.Client, clientID, region, email, code string) error {
	body := map[string]any{
		"ClientId":         clientID,
		"Username":         email,
		"ConfirmationCode": code,
	}
	_, err := cognitoRequest(ctx, hc, region, "AWSCognitoIdentityProviderService.ConfirmSignUp", body)
	return err
}

func cognitoForgotPassword(ctx context.Context, hc *http.Client, clientID, region, email string) error {
	body := map[string]any{
		"ClientId": clientID,
		"Username": email,
	}
	_, err := cognitoRequest(ctx, hc, region, "AWSCognitoIdentityProviderService.ForgotPassword", body)
	return err
}

func cognitoConfirmForgotPassword(ctx context.Context, hc *http.Client, clientID, region, email, code, newPassword string) error {
	body := map[string]any{
		"ClientId":         clientID,
		"Username":         email,
		"ConfirmationCode": code,
		"Password":         newPassword,
	}
	_, err := cognitoRequest(ctx, hc, region, "AWSCognitoIdentityProviderService.ConfirmForgotPassword", body)
	return err
}

func cognitoRequest(ctx context.Context, hc *http.Client, region, target string, body map[string]any) (map[string]any, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("attago: marshal cognito request: %w", err)
	}

	endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", region)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("attago: build cognito request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", target)

	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("attago: cognito request failed: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("attago: read cognito response: %w", err)
	}

	if res.StatusCode >= 400 {
		var errBody map[string]any
		_ = json.Unmarshal(respBytes, &errBody)
		msg, _ := errBody["message"].(string)
		code, _ := errBody["__type"].(string)
		return nil, &AuthError{Message: msg, Code: code}
	}

	var result map[string]any
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("attago: decode cognito response: %w", err)
	}

	return result, nil
}

func extractTokens(result map[string]any) (*CognitoTokens, error) {
	authResult, ok := result["AuthenticationResult"].(map[string]any)
	if !ok {
		return nil, &AuthError{Message: "Missing AuthenticationResult in Cognito response"}
	}
	idToken, _ := authResult["IdToken"].(string)
	accessToken, _ := authResult["AccessToken"].(string)
	refreshToken, _ := authResult["RefreshToken"].(string)
	if idToken == "" {
		return nil, &AuthError{Message: "Missing IdToken in Cognito response"}
	}
	return &CognitoTokens{
		IDToken:      idToken,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
