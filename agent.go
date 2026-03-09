package attago

import (
	"context"
	"strings"
)

// AgentService provides access to agent scoring endpoints.
type AgentService struct {
	client *Client
}

// GetScore returns the Go/No-Go score for a single asset.
func (s *AgentService) GetScore(ctx context.Context, symbol string) (*AgentScoreResponse, error) {
	var result AgentScoreResponse
	err := s.client.do(ctx, "GET", "/agent/score", &result, WithQuery("symbol", symbol))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetData returns full market data for all tracked assets.
// Pass symbols to filter, or nil/empty for all.
func (s *AgentService) GetData(ctx context.Context, symbols ...string) (*AgentDataResponse, error) {
	var result AgentDataResponse
	var opts []RequestOption
	if len(symbols) > 0 {
		opts = append(opts, WithQuery("symbols", strings.Join(symbols, ",")))
	}
	err := s.client.do(ctx, "GET", "/agent/data", &result, opts...)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

