package attago

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
)

// WebhookListener is a local HTTP server for receiving AttaGo webhook deliveries.
// It verifies X-AttaGo-Signature (HMAC-SHA256), parses v2 payloads, and
// calls registered handlers.
type WebhookListener struct {
	Port   int
	Host   string
	Path   string
	secret string

	server *http.Server
	ln     net.Listener

	mu       sync.RWMutex
	onAlert  []func(WebhookPayload)
	onTest   []func(WebhookPayload)
	onError  []func(error)
}

// WebhookListenerConfig configures a WebhookListener.
type WebhookListenerConfig struct {
	// Secret is the HMAC signing secret (from webhook creation response).
	Secret string
	// Port to listen on. Defaults to 4000. Use 0 for OS-assigned port.
	Port int
	// Host to bind to. Defaults to "0.0.0.0".
	Host string
	// Path to accept webhooks on. Defaults to "/webhook".
	Path string
}

// NewWebhookListener creates a new webhook listener.
func NewWebhookListener(cfg WebhookListenerConfig) *WebhookListener {
	port := cfg.Port
	if port == 0 {
		port = 4000
	}
	host := cfg.Host
	if host == "" {
		host = "0.0.0.0"
	}
	path := cfg.Path
	if path == "" {
		path = "/webhook"
	}
	return &WebhookListener{
		Port:   port,
		Host:   host,
		Path:   path,
		secret: cfg.Secret,
	}
}

// OnAlert registers a handler for alert events.
func (l *WebhookListener) OnAlert(handler func(WebhookPayload)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onAlert = append(l.onAlert, handler)
}

// OnTest registers a handler for test events.
func (l *WebhookListener) OnTest(handler func(WebhookPayload)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onTest = append(l.onTest, handler)
}

// OnError registers a handler for errors.
func (l *WebhookListener) OnError(handler func(error)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onError = append(l.onError, handler)
}

// Start begins listening for webhooks. Returns when the server is ready.
// Use Addr() to get the actual listening address (useful with port 0).
func (l *WebhookListener) Start() error {
	l.mu.Lock()
	if l.server != nil {
		l.mu.Unlock()
		return fmt.Errorf("listener already started")
	}
	l.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", l.Host, l.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("attago: listen: %w", err)
	}

	// Update Port to the actual port (for port 0)
	l.Port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc(l.Path, l.handleWebhook)

	srv := &http.Server{Handler: mux}

	l.mu.Lock()
	l.server = srv
	l.ln = ln
	l.mu.Unlock()

	// Use the local srv variable so the goroutine never races with Stop()
	// setting l.server = nil.
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			l.emitError(err)
		}
	}()

	return nil
}

// Stop shuts down the HTTP server.
func (l *WebhookListener) Stop(ctx context.Context) error {
	l.mu.Lock()
	srv := l.server
	l.server = nil
	l.ln = nil
	l.mu.Unlock()

	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// Addr returns the listener's address (e.g. "0.0.0.0:4000").
// Useful after Start() with port 0 to discover the actual port.
func (l *WebhookListener) Addr() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.ln == nil {
		return ""
	}
	return l.ln.Addr().String()
}

// Listening returns whether the server is currently running.
func (l *WebhookListener) Listening() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.server != nil
}

// ── HTTP handler ────────────────────────────────────────────────────

func (l *WebhookListener) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		l.emitError(fmt.Errorf("read body: %w", err))
		writeJSON(w, 400, map[string]string{"error": "Invalid payload"})
		return
	}

	// Verify signature
	signature := r.Header.Get("X-AttaGo-Signature")
	if signature == "" || !VerifySignature(rawBody, l.secret, signature) {
		writeJSON(w, 401, map[string]string{"error": "Invalid signature"})
		return
	}

	// Parse payload
	var payload WebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		l.emitError(fmt.Errorf("parse payload: %w", err))
		writeJSON(w, 400, map[string]string{"error": "Invalid payload"})
		return
	}

	// Emit event
	l.mu.RLock()
	var handlers []func(WebhookPayload)
	if payload.Event == "test" {
		handlers = l.onTest
	} else {
		handlers = l.onAlert
	}
	l.mu.RUnlock()

	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					l.emitError(fmt.Errorf("handler panic: %v", r))
				}
			}()
			handler(payload)
		}()
	}

	writeJSON(w, 200, map[string]string{"received": "true"})
}

func (l *WebhookListener) emitError(err error) {
	l.mu.RLock()
	handlers := l.onError
	l.mu.RUnlock()

	for _, handler := range handlers {
		func() {
			defer func() { recover() }() // swallow panics in error handlers
			handler(err)
		}()
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}
