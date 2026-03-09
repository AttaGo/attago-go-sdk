package attago

import "context"

// RedeemService provides access to redemption code endpoints.
type RedeemService struct {
	client *Client
}

// Redeem activates a redemption code, granting tier access for the specified duration.
func (s *RedeemService) Redeem(ctx context.Context, code string) (*RedeemResponse, error) {
	var result RedeemResponse
	err := s.client.do(ctx, "POST", "/user/redeem", &result, WithBody(map[string]string{"code": code}))
	if err != nil {
		return nil, err
	}
	return &result, nil
}
