package attago

import (
	"context"
	"net/url"
)

// WalletService provides access to wallet management endpoints.
type WalletService struct {
	client *Client
}

// Register registers and verifies a wallet via challenge-response.
func (s *WalletService) Register(ctx context.Context, input RegisterWalletInput) (*Wallet, error) {
	var result Wallet
	err := s.client.do(ctx, "POST", "/payments/wallet", &result, WithBody(input))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns all verified wallets for the current user.
func (s *WalletService) List(ctx context.Context) ([]Wallet, error) {
	var result struct {
		Wallets []Wallet `json:"wallets"`
	}
	err := s.client.do(ctx, "GET", "/payments/wallets", &result)
	if err != nil {
		return nil, err
	}
	return result.Wallets, nil
}

// Remove unregisters a verified wallet.
func (s *WalletService) Remove(ctx context.Context, address string) error {
	return s.client.do(ctx, "DELETE", "/payments/wallet/"+url.PathEscape(address), nil)
}
