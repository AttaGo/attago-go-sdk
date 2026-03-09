package attago

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

// MCPService provides access to the MCP (Model Context Protocol) server.
// Uses JSON-RPC 2.0 over HTTP POST to /v1/mcp.
type MCPService struct {
	client *Client
	nextID atomic.Int64
}

// Initialize negotiates the protocol version with the MCP server.
func (s *MCPService) Initialize(ctx context.Context) (*MCPServerInfo, error) {
	var result MCPServerInfo
	err := s.rpc(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTools discovers available tools and their schemas.
func (s *MCPService) ListTools(ctx context.Context) ([]MCPTool, error) {
	var result struct {
		Tools []MCPTool `json:"tools"`
	}
	err := s.rpc(ctx, "tools/list", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool executes an MCP tool.
func (s *MCPService) CallTool(ctx context.Context, name string, args map[string]any) (*MCPToolCallResult, error) {
	if args == nil {
		args = map[string]any{}
	}
	var result MCPToolCallResult
	err := s.rpc(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Ping sends a health check to the MCP server.
func (s *MCPService) Ping(ctx context.Context) error {
	var result map[string]any
	return s.rpc(ctx, "ping", nil, &result)
}

// ── JSON-RPC 2.0 internals ─────────────────────────────────────────

type jsonRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (s *MCPService) rpc(ctx context.Context, method string, params map[string]any, result any) error {
	requestID := s.nextID.Add(1)

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  method,
		Params:  params,
	}

	bodyBytes, err := json.Marshal(rpcReq)
	if err != nil {
		return fmt.Errorf("attago: marshal MCP request: %w", err)
	}

	mcpURL := s.client.baseURL + "/v1/mcp"
	req, err := http.NewRequestWithContext(ctx, "POST", mcpURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("attago: build MCP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "attago-go-sdk/"+Version)

	// Auth headers
	switch s.client.AuthMode() {
	case AuthModeAPIKey:
		req.Header.Set("X-API-Key", s.client.apiKey)
	case AuthModeCognito:
		if s.client.Auth != nil {
			token, tokenErr := s.client.Auth.GetIDToken(ctx)
			if tokenErr != nil {
				return tokenErr
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	// Execute (with x402 for signer mode)
	var res *http.Response
	if s.client.AuthMode() == AuthModeX402 && s.client.signer != nil {
		res, err = doWithX402(ctx, s.client.httpClient, s.client.signer, req, bodyBytes)
	} else {
		res, err = s.client.httpClient.Do(req)
	}
	if err != nil {
		return fmt.Errorf("attago: MCP request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		respBody, _ := io.ReadAll(res.Body)
		return &MCPError{
			Code:    res.StatusCode,
			Message: fmt.Sprintf("HTTP %d: %s", res.StatusCode, string(respBody)),
		}
	}

	var rpcRes jsonRPCResponse
	if err := json.NewDecoder(res.Body).Decode(&rpcRes); err != nil {
		return fmt.Errorf("attago: decode MCP response: %w", err)
	}

	if rpcRes.Error != nil {
		return &MCPError{
			Code:    rpcRes.Error.Code,
			Message: rpcRes.Error.Message,
			Data:    rpcRes.Error.Data,
		}
	}

	if result != nil && rpcRes.Result != nil {
		if err := json.Unmarshal(rpcRes.Result, result); err != nil {
			return fmt.Errorf("attago: unmarshal MCP result: %w", err)
		}
	}

	return nil
}
