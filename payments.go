package attago

import "context"

// PaymentService provides access to billing endpoints.
type PaymentService struct {
	client *Client
}

// Subscribe purchases or activates a subscription tier.
func (s *PaymentService) Subscribe(ctx context.Context, input SubscribeInput) (*SubscribeResponse, error) {
	var result SubscribeResponse
	err := s.client.do(ctx, "POST", "/payments/subscribe", &result, WithBody(input))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Status returns the current billing status, tier, and push usage.
func (s *PaymentService) Status(ctx context.Context) (*BillingStatus, error) {
	var result BillingStatus
	err := s.client.do(ctx, "GET", "/payments/status", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpgradeQuote returns a pro-rated upgrade price quote.
func (s *PaymentService) UpgradeQuote(ctx context.Context, tier, cycle string) (*UpgradeQuote, error) {
	var result UpgradeQuote
	err := s.client.do(ctx, "GET", "/payments/upgrade-quote", &result,
		WithQuery("tier", tier),
		WithQuery("cycle", cycle),
	)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
