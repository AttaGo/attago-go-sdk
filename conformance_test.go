//go:build conformance

package attago

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Fixture types ────────────────────────────────────────────────────

type fixtureRequest struct {
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Headers        map[string]string `json:"headers"`
	Query          map[string]string `json:"query"`
	PathParameters map[string]string `json:"pathParameters"`
	Body           any               `json:"body"`
}

type fixtureResponse struct {
	Status int    `json:"status"`
	Schema string `json:"schema"`
}

type fixture struct {
	Description string          `json:"description"`
	Request     fixtureRequest  `json:"request"`
	Response    fixtureResponse `json:"response"`
}

// ── Schema types (structural validation) ─────────────────────────────

type jsonSchema struct {
	Required   []string                `json:"required"`
	Properties map[string]jsonSchema   `json:"properties"`
	Type       any                     `json:"type"` // string or []string
	Items      *jsonSchema             `json:"items"`
}

// shouldSkip returns true for fixtures that can't run in CI:
// - JWT fixtures (Authorization: Bearer) — need real Cognito tokens
// - Unauthorized tests (expect 401 with no auth) — dev API may not enforce
func shouldSkip(fx fixture) bool {
	// Skip JWT-auth fixtures (CI only has API keys, not Cognito tokens)
	if _, hasAuth := fx.Request.Headers["Authorization"]; hasAuth {
		return true
	}
	// Skip fixtures that test auth enforcement (expect 4xx with no auth)
	if fx.Response.Status == 401 {
		if _, hasKey := fx.Request.Headers["X-API-Key"]; !hasKey {
			return true
		}
	}
	return false
}

// ── Runner ───────────────────────────────────────────────────────────

func TestConformance(t *testing.T) {
	baseURL := os.Getenv("ATTAGO_BASE_URL")
	if baseURL == "" {
		t.Skip("ATTAGO_BASE_URL not set — skipping conformance tests")
	}

	apiKey := os.Getenv("ATTAGO_API_KEY")
	if apiKey == "" {
		t.Skip("ATTAGO_API_KEY not set — skipping conformance tests")
	}

	specDir := os.Getenv("ATTAGO_SPEC_DIR")
	if specDir == "" {
		// Default: sibling directory
		specDir = filepath.Join("..", "attago-spec")
	}

	fixtureDir := filepath.Join(specDir, "spec", "fixtures", "rest")
	schemaDir := filepath.Join(specDir, "spec", "schema")

	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Skipf("fixture dir not found: %s — skipping", fixtureDir)
	}

	// Load schemas (extract required fields for structural validation)
	schemas := loadSchemas(t, schemaDir)

	// Load and run fixtures
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixtureDir, name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var fx fixture
			if err := json.Unmarshal(data, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			// Auto-skip JWT fixtures and unauthorized tests
			if shouldSkip(fx) {
				t.Skip("auto-skipped: JWT auth or unauthorized test")
			}

			// Substitute path parameters (e.g. {id}) before building URL
			resolvedPath := fx.Request.Path
			for k, v := range fx.Request.PathParameters {
				resolvedPath = strings.ReplaceAll(resolvedPath, "{"+k+"}", v)
			}
			url := strings.TrimRight(baseURL, "/") + resolvedPath
			if len(fx.Request.Query) > 0 {
				params := make([]string, 0, len(fx.Request.Query))
				for k, v := range fx.Request.Query {
					params = append(params, k+"="+v)
				}
				url += "?" + strings.Join(params, "&")
			}

			// Build request
			var bodyReader io.Reader
			if fx.Request.Body != nil {
				bodyBytes, _ := json.Marshal(fx.Request.Body)
				bodyReader = strings.NewReader(string(bodyBytes))
			}

			req, err := http.NewRequest(fx.Request.Method, url, bodyReader)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			req.Header.Set("Accept", "application/json")

			for k, v := range fx.Request.Headers {
				// Inject real API key
				if k == "X-API-Key" && apiKey != "" {
					req.Header.Set(k, apiKey)
				} else {
					req.Header.Set(k, v)
				}
			}
			if fx.Request.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			// Send request
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("HTTP request failed: %v", err)
			}
			defer res.Body.Close()

			// Validate status code
			if res.StatusCode != fx.Response.Status {
				body, _ := io.ReadAll(res.Body)
				t.Fatalf("status = %d, want %d\nbody: %s", res.StatusCode, fx.Response.Status, body)
			}

			// Validate required fields (structural validation, no full JSON Schema)
			if fx.Response.Schema != "" && res.StatusCode < 400 {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					t.Fatalf("read response body: %v", err)
				}

				schema, ok := schemas[fx.Response.Schema]
				if !ok {
					t.Logf("WARNING: schema %q not found, skipping structural check", fx.Response.Schema)
					return
				}

				var parsed map[string]any
				if err := json.Unmarshal(body, &parsed); err != nil {
					t.Fatalf("response is not valid JSON: %v", err)
				}

				validateRequired(t, parsed, schema, "")
			}
		})
	}
}

// ── Schema loader ────────────────────────────────────────────────────

func loadSchemas(t *testing.T, dir string) map[string]jsonSchema {
	t.Helper()
	schemas := make(map[string]jsonSchema)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Logf("schema dir not found: %s — structural validation disabled", dir)
		return schemas
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Logf("read schema dir: %v — structural validation disabled", err)
		return schemas
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Logf("read schema %s: %v", entry.Name(), err)
			continue
		}

		var s jsonSchema
		if err := json.Unmarshal(data, &s); err != nil {
			t.Logf("parse schema %s: %v", entry.Name(), err)
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".schema.json")
		schemas[name] = s
	}

	return schemas
}

// ── Structural validator ─────────────────────────────────────────────
// Checks that all "required" fields from the schema exist in the response.
// Recursively walks nested objects. No type checking (full JSON Schema
// validation would need a third-party library — we keep zero deps).

func validateRequired(t *testing.T, obj map[string]any, schema jsonSchema, path string) {
	t.Helper()

	for _, field := range schema.Required {
		fieldPath := path + "." + field
		if path == "" {
			fieldPath = field
		}

		val, exists := obj[field]
		if !exists {
			t.Errorf("missing required field: %s", fieldPath)
			continue
		}

		// Recurse into nested objects if schema defines properties
		if propSchema, ok := schema.Properties[field]; ok {
			if nested, ok := val.(map[string]any); ok && len(propSchema.Required) > 0 {
				validateRequired(t, nested, propSchema, fieldPath)
			}
		}
	}
}

// ── MCP conformance ──────────────────────────────────────────────────

func TestConformance_MCP(t *testing.T) {
	baseURL := os.Getenv("ATTAGO_BASE_URL")
	if baseURL == "" {
		t.Skip("ATTAGO_BASE_URL not set — skipping conformance tests")
	}

	specDir := os.Getenv("ATTAGO_SPEC_DIR")
	if specDir == "" {
		specDir = filepath.Join("..", "attago-spec")
	}

	fixtureDir := filepath.Join(specDir, "spec", "fixtures", "mcp")
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Skipf("MCP fixture dir not found: %s — skipping", fixtureDir)
	}

	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read MCP fixture dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixtureDir, name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var fx fixture
			if err := json.Unmarshal(data, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			// MCP fixtures always POST to /v1/mcp
			url := strings.TrimRight(baseURL, "/") + fx.Request.Path

			var bodyReader io.Reader
			if fx.Request.Body != nil {
				bodyBytes, _ := json.Marshal(fx.Request.Body)
				bodyReader = strings.NewReader(string(bodyBytes))
			}

			req, err := http.NewRequest("POST", url, bodyReader)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")

			for k, v := range fx.Request.Headers {
				req.Header.Set(k, v)
			}

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("HTTP request failed: %v", err)
			}
			defer res.Body.Close()

			if res.StatusCode != fx.Response.Status {
				body, _ := io.ReadAll(res.Body)
				t.Fatalf("status = %d, want %d\nbody: %s", res.StatusCode, fx.Response.Status, body)
			}

			// For 200 responses, verify it's valid JSON-RPC 2.0
			if res.StatusCode == 200 {
				body, _ := io.ReadAll(res.Body)
				var rpcResp map[string]any
				if err := json.Unmarshal(body, &rpcResp); err != nil {
					t.Fatalf("response is not valid JSON: %v", err)
				}

				if v, ok := rpcResp["jsonrpc"]; !ok || v != "2.0" {
					t.Errorf("jsonrpc = %v, want 2.0", v)
				}
				if _, ok := rpcResp["result"]; !ok {
					if _, hasErr := rpcResp["error"]; !hasErr {
						t.Error("response has neither 'result' nor 'error'")
					}
				}
			}
		})
	}
}

// ── x402 fixture conformance ─────────────────────────────────────────

func TestConformance_X402(t *testing.T) {
	baseURL := os.Getenv("ATTAGO_BASE_URL")
	if baseURL == "" {
		t.Skip("ATTAGO_BASE_URL not set — skipping conformance tests")
	}

	specDir := os.Getenv("ATTAGO_SPEC_DIR")
	if specDir == "" {
		specDir = filepath.Join("..", "attago-spec")
	}

	fixtureDir := filepath.Join(specDir, "spec", "fixtures", "x402")
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Skipf("x402 fixture dir not found: %s — skipping", fixtureDir)
	}

	// Only test the 402 response shape (no wallet needed)
	data, err := os.ReadFile(filepath.Join(fixtureDir, "payment-required-402.json"))
	if err != nil {
		t.Skipf("payment-required-402.json not found: %v", err)
	}

	var fx fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	// Skip if fixture uses JWT auth (CI only has API keys)
	if shouldSkip(fx) {
		t.Skip("auto-skipped: fixture requires JWT auth")
	}

	url := strings.TrimRight(baseURL, "/") + fx.Request.Path

	var bodyReader io.Reader
	if fx.Request.Body != nil {
		bodyBytes, _ := json.Marshal(fx.Request.Body)
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	req, err := http.NewRequest(fx.Request.Method, url, bodyReader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range fx.Request.Headers {
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer res.Body.Close()

	// Should be 402
	if res.StatusCode != 402 {
		t.Fatalf("status = %d, want 402", res.StatusCode)
	}

	// Must have Payment-Required header (base64-encoded JSON)
	prHeader := res.Header.Get("Payment-Required")
	if prHeader == "" {
		t.Fatal("missing Payment-Required header on 402 response")
	}

	// Try to parse via our SDK function
	reqs := ParsePaymentRequired(res.Header)
	if reqs == nil {
		t.Fatal("ParsePaymentRequired returned nil for valid 402")
	}

	if reqs.X402Version < 1 {
		t.Errorf("X402Version = %d, want >= 1", reqs.X402Version)
	}
	if len(reqs.Accepts) == 0 {
		t.Error("Accepts is empty — expected at least one payment option")
	}

	// Validate each accepted payment has required fields
	for i, accept := range reqs.Accepts {
		prefix := fmt.Sprintf("Accepts[%d]", i)
		if accept.Network == "" {
			t.Errorf("%s.Network is empty", prefix)
		}
		if accept.Amount == "" {
			t.Errorf("%s.Amount is empty", prefix)
		}
	}
}
