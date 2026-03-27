package attago

import "context"

// MessagingService provides access to messaging platform link/unlink and test delivery.
type MessagingService struct {
	client *Client
}

// List returns connected messaging platforms.
func (s *MessagingService) List(ctx context.Context) ([]MessagingLink, error) {
	var result struct {
		Items []MessagingLink `json:"items"`
	}
	err := s.client.do(ctx, "GET", "/user/messaging", &result)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// LinkTelegram connects a Telegram account using a 6-digit code from the bot.
// Requires Pro+ tier.
func (s *MessagingService) LinkTelegram(ctx context.Context, code string) (*LinkResult, error) {
	var result LinkResult
	err := s.client.do(ctx, "POST", "/user/messaging/telegram/link", &result, WithBody(map[string]string{"code": code}))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UnlinkTelegram disconnects the Telegram account.
func (s *MessagingService) UnlinkTelegram(ctx context.Context) (*UnlinkResult, error) {
	var result UnlinkResult
	err := s.client.do(ctx, "DELETE", "/user/messaging/telegram", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Test sends a test message to all connected messaging platforms.
// Requires Pro+ tier. Subject to 30-second cooldown.
func (s *MessagingService) Test(ctx context.Context) (*MessagingTestResult, error) {
	var result MessagingTestResult
	err := s.client.do(ctx, "POST", "/user/messaging/test", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
