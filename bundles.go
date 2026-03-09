package attago

import "context"

// BundleService provides access to data-push credit bundle endpoints.
type BundleService struct {
	client *Client
}

// List returns purchased bundles and the available catalog.
func (s *BundleService) List(ctx context.Context) (*BundleListResponse, error) {
	var result BundleListResponse
	err := s.client.do(ctx, "GET", "/api/bundles", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Purchase buys a data-push credit bundle.
// In live mode, triggers a 402 x402 payment flow.
func (s *BundleService) Purchase(ctx context.Context, input PurchaseBundleInput) (*BundlePurchaseResponse, error) {
	var result BundlePurchaseResponse
	err := s.client.do(ctx, "POST", "/api/bundles", &result, WithBody(input))
	if err != nil {
		return nil, err
	}
	return &result, nil
}
