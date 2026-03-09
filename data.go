package attago

import (
	"context"
	"net/url"
)

// DataService provides access to data endpoints.
type DataService struct {
	client *Client
}

// GetLatest returns latest market data for all tracked assets at the user's tier.
func (s *DataService) GetLatest(ctx context.Context) (*DataLatestResponse, error) {
	var result DataLatestResponse
	err := s.client.do(ctx, "GET", "/data/latest", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTokenData returns derived scoring for a single token.
// Paid via x402, bundle deduction, or included monthly pushes.
func (s *DataService) GetTokenData(ctx context.Context, token string) (*DataTokenResponse, error) {
	var result DataTokenResponse
	err := s.client.do(ctx, "GET", "/api/data/"+url.PathEscape(token), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDataPush retrieves a 72-hour snapshot by request ID.
// No authentication needed — the requestId serves as a bearer token.
func (s *DataService) GetDataPush(ctx context.Context, requestID string) (*DataPushResponse, error) {
	var result DataPushResponse
	err := s.client.do(ctx, "GET", "/data/push/"+url.PathEscape(requestID), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
