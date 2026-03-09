package attago

import (
	"context"
	"net/url"
)

// SubscriptionService provides access to alert subscription endpoints.
type SubscriptionService struct {
	client *Client
}

// Catalog returns available tokens, metrics, and tier limits.
func (s *SubscriptionService) Catalog(ctx context.Context) (*CatalogResponse, error) {
	var result CatalogResponse
	err := s.client.do(ctx, "GET", "/subscriptions/catalog", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns the user's active subscriptions.
func (s *SubscriptionService) List(ctx context.Context) ([]Subscription, error) {
	var result struct {
		Subscriptions []Subscription `json:"subscriptions"`
	}
	err := s.client.do(ctx, "GET", "/user/subscriptions", &result)
	if err != nil {
		return nil, err
	}
	return result.Subscriptions, nil
}

// Create creates a new alert subscription.
func (s *SubscriptionService) Create(ctx context.Context, input CreateSubscriptionInput) (*Subscription, error) {
	var result Subscription
	err := s.client.do(ctx, "POST", "/user/subscriptions", &result, WithBody(input))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Update updates an existing subscription. Only non-nil fields are sent.
func (s *SubscriptionService) Update(ctx context.Context, subID string, input UpdateSubscriptionInput) (*Subscription, error) {
	var result Subscription
	err := s.client.do(ctx, "PUT", "/user/subscriptions/"+url.PathEscape(subID), &result, WithBody(input))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete removes a subscription.
func (s *SubscriptionService) Delete(ctx context.Context, subID string) error {
	return s.client.do(ctx, "DELETE", "/user/subscriptions/"+url.PathEscape(subID), nil)
}
