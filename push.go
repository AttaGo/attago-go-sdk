package attago

import (
	"context"
	"net/url"
)

// PushService provides access to Web Push notification subscription endpoints.
type PushService struct {
	client *Client
}

// List returns registered push subscriptions.
func (s *PushService) List(ctx context.Context) ([]PushSubscriptionResponse, error) {
	var result struct {
		Items []PushSubscriptionResponse `json:"items"`
	}
	err := s.client.do(ctx, "GET", "/user/push-subscriptions", &result)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Create registers a Web Push subscription.
func (s *PushService) Create(ctx context.Context, input CreatePushInput) (*PushSubscriptionResponse, error) {
	var result PushSubscriptionResponse
	err := s.client.do(ctx, "POST", "/user/push-subscriptions", &result, WithBody(input))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete removes a push subscription.
func (s *PushService) Delete(ctx context.Context, subscriptionID string) error {
	return s.client.do(ctx, "DELETE", "/user/push-subscriptions/"+url.PathEscape(subscriptionID), nil)
}
