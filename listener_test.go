package attago

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestWebhookListener_StartStop(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{
		Secret: "test-secret",
		Port:   0, // OS-assigned
	})

	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	if !l.Listening() {
		t.Error("should be listening after Start")
	}
	if l.Port == 0 {
		t.Error("Port should be assigned after Start with port 0")
	}
	addr := l.Addr()
	if addr == "" {
		t.Error("Addr() should not be empty")
	}
}

func TestWebhookListener_DoubleStart_Error(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{Secret: "s", Port: 0})
	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	if err := l.Start(); err == nil {
		t.Error("expected error on double start")
	}
}

func TestWebhookListener_ValidSignature_200(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{
		Secret: "wh_secret_test",
		Port:   0,
	})

	var received WebhookPayload
	var mu sync.Mutex

	l.OnAlert(func(p WebhookPayload) {
		mu.Lock()
		defer mu.Unlock()
		received = p
	})

	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	// Build and sign a test payload
	payload := BuildTestPayload("BTC", "triggered", "production", "attago.bid")
	payload["event"] = "alert" // alert, not test
	bodyBytes, _ := json.Marshal(payload)
	sig := SignPayload(bodyBytes, "wh_secret_test")

	url := fmt.Sprintf("http://%s/webhook", l.Addr())
	req, _ := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AttaGo-Signature", sig)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}

	// Wait for handler
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if received.Alert.Token != "BTC" {
		t.Errorf("received token = %q", received.Alert.Token)
	}
	mu.Unlock()
}

func TestWebhookListener_InvalidSignature_401(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{Secret: "correct-secret", Port: 0})
	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	body := []byte(`{"event":"alert"}`)
	url := fmt.Sprintf("http://%s/webhook", l.Addr())
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("X-AttaGo-Signature", "wrong-signature")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 401 {
		t.Errorf("status = %d, want 401", res.StatusCode)
	}
}

func TestWebhookListener_MissingSignature_401(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{Secret: "secret", Port: 0})
	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	url := fmt.Sprintf("http://%s/webhook", l.Addr())
	res, err := http.Post(url, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 401 {
		t.Errorf("status = %d, want 401", res.StatusCode)
	}
}

func TestWebhookListener_WrongPath_404(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{Secret: "secret", Port: 0})
	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	url := fmt.Sprintf("http://%s/wrong-path", l.Addr())
	res, err := http.Post(url, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 404 {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}

func TestWebhookListener_TestEvent_RoutesToOnTest(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{Secret: "secret", Port: 0})

	alertCalled := false
	testCalled := false
	var mu sync.Mutex

	l.OnAlert(func(_ WebhookPayload) {
		mu.Lock()
		alertCalled = true
		mu.Unlock()
	})
	l.OnTest(func(_ WebhookPayload) {
		mu.Lock()
		testCalled = true
		mu.Unlock()
	})

	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	payload := BuildTestPayload("BTC", "triggered", "production", "")
	bodyBytes, _ := json.Marshal(payload)
	sig := SignPayload(bodyBytes, "secret")

	url := fmt.Sprintf("http://%s/webhook", l.Addr())
	req, _ := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AttaGo-Signature", sig)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	res.Body.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if alertCalled {
		t.Error("OnAlert should not be called for test events")
	}
	if !testCalled {
		t.Error("OnTest should be called for test events")
	}
	mu.Unlock()
}

func TestWebhookListener_OnError_HandlerPanic(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{Secret: "secret", Port: 0})

	var capturedErr error
	var mu sync.Mutex

	l.OnAlert(func(_ WebhookPayload) {
		panic("handler exploded")
	})
	l.OnError(func(err error) {
		mu.Lock()
		capturedErr = err
		mu.Unlock()
	})

	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	payload := map[string]any{"event": "alert", "alert": map[string]any{"token": "BTC"}}
	bodyBytes, _ := json.Marshal(payload)
	sig := SignPayload(bodyBytes, "secret")

	url := fmt.Sprintf("http://%s/webhook", l.Addr())
	req, _ := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("X-AttaGo-Signature", sig)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	res.Body.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if capturedErr == nil {
		t.Error("OnError should capture handler panics")
	}
	mu.Unlock()
}

func TestWebhookListener_CustomPath(t *testing.T) {
	l := NewWebhookListener(WebhookListenerConfig{
		Secret: "secret",
		Port:   0,
		Path:   "/custom/hook",
	})
	if err := l.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer l.Stop(context.Background())

	body := []byte(`{"event":"test"}`)
	sig := SignPayload(body, "secret")

	// Wrong path = 404
	url404 := fmt.Sprintf("http://%s/webhook", l.Addr())
	res404, _ := http.Post(url404, "application/json", bytes.NewReader(body))
	res404.Body.Close()
	if res404.StatusCode != 404 {
		t.Errorf("wrong path status = %d, want 404", res404.StatusCode)
	}

	// Correct custom path = 200
	urlOK := fmt.Sprintf("http://%s/custom/hook", l.Addr())
	req, _ := http.NewRequest("POST", urlOK, bytes.NewReader(body))
	req.Header.Set("X-AttaGo-Signature", sig)
	resOK, _ := http.DefaultClient.Do(req)
	resOK.Body.Close()
	if resOK.StatusCode != 200 {
		t.Errorf("correct path status = %d, want 200", resOK.StatusCode)
	}
}
