package attago

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPService_Initialize(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/mcp" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("method = %q", r.Method)
		}

		var rpc jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&rpc)
		if rpc.Method != "initialize" {
			t.Errorf("rpc method = %q", rpc.Method)
		}
		if rpc.JSONRPC != "2.0" {
			t.Errorf("jsonrpc = %q", rpc.JSONRPC)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      rpc.ID,
			"result": map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities":   map[string]any{"tools": map[string]any{"listChanged": false}},
				"serverInfo":     map[string]any{"name": "AttaGo MCP", "version": "1.0.0"},
				"instructions":   "Use get_asset_score to fetch scores",
			},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.MCP.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}
	if result.ProtocolVersion != "2025-03-26" {
		t.Errorf("ProtocolVersion = %q", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "AttaGo MCP" {
		t.Errorf("ServerInfo.Name = %q", result.ServerInfo.Name)
	}
}

func TestMCPService_ListTools(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&rpc)
		if rpc.Method != "tools/list" {
			t.Errorf("rpc method = %q", rpc.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      rpc.ID,
			"result": map[string]any{
				"tools": []map[string]any{
					{"name": "get_asset_score", "description": "Get score for an asset"},
					{"name": "list_assets", "description": "List available assets"},
				},
			},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	tools, err := c.MCP.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("len = %d, want 2", len(tools))
	}
	if tools[0].Name != "get_asset_score" {
		t.Errorf("tools[0].Name = %q", tools[0].Name)
	}
}

func TestMCPService_CallTool(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&rpc)
		if rpc.Method != "tools/call" {
			t.Errorf("rpc method = %q", rpc.Method)
		}
		name, _ := rpc.Params["name"].(string)
		if name != "get_asset_score" {
			t.Errorf("tool name = %q", name)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      rpc.ID,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"score": 72, "signal": "GO"}`},
				},
				"isError": false,
			},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	result, err := c.MCP.CallTool(context.Background(), "get_asset_score", map[string]any{"symbol": "BTC"})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if result.IsError {
		t.Error("IsError should be false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("content length = %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q", result.Content[0].Type)
	}
}

func TestMCPService_Ping(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&rpc)
		if rpc.Method != "ping" {
			t.Errorf("method = %q", rpc.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": rpc.ID, "result": map[string]any{},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	err := c.MCP.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestMCPService_JsonRpcError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&rpc)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      rpc.ID,
			"error":   map[string]any{"code": -32601, "message": "Method not found"},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	_, err := c.MCP.CallTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var mcpErr *MCPError
	if !errors.As(err, &mcpErr) {
		t.Fatalf("expected *MCPError, got %T", err)
	}
	if mcpErr.Code != -32601 {
		t.Errorf("Code = %d", mcpErr.Code)
	}
}

func TestMCPService_AutoIncrementIDs(t *testing.T) {
	ids := []int64{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&rpc)
		ids = append(ids, rpc.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": rpc.ID, "result": map[string]any{},
		})
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("ak_test"))
	c.MCP.Ping(context.Background())
	c.MCP.Ping(context.Background())
	c.MCP.Ping(context.Background())

	if len(ids) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(ids))
	}
	// IDs should be sequential
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("IDs not sequential: %v", ids)
			break
		}
	}
}
