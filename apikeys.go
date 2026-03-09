package attago

import (
	"context"
	"net/url"
)

// APIKeyService provides access to API key management endpoints.
type APIKeyService struct {
	client *Client
}

// Create creates a new API key. Requires Cognito JWT authentication.
// The Key in the response is shown only once — save it immediately.
func (s *APIKeyService) Create(ctx context.Context, name string) (*APIKeyCreateResponse, error) {
	var result APIKeyCreateResponse
	err := s.client.do(ctx, "POST", "/user/api-keys", &result, WithBody(map[string]string{"name": name}))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns all API keys (active and revoked, for audit trail).
func (s *APIKeyService) List(ctx context.Context) ([]APIKeyListItem, error) {
	var result struct {
		Items []APIKeyListItem `json:"items"`
	}
	err := s.client.do(ctx, "GET", "/user/api-keys", &result)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Revoke soft-revokes an API key. The key immediately stops working.
func (s *APIKeyService) Revoke(ctx context.Context, keyID string) error {
	return s.client.do(ctx, "DELETE", "/user/api-keys/"+url.PathEscape(keyID), nil)
}
