package attago

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ParsePaymentRequired decodes the base64-encoded PAYMENT-REQUIRED header
// into structured x402 requirements. Returns nil if the header is missing
// or unparseable.
func ParsePaymentRequired(headers http.Header) *X402PaymentRequirements {
	raw := headers.Get("Payment-Required")
	if raw == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// Try URL-safe encoding
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return nil
		}
	}
	var reqs X402PaymentRequirements
	if err := json.Unmarshal(decoded, &reqs); err != nil {
		return nil
	}
	return &reqs
}

// FilterAcceptsByNetwork returns the first accepted payment option matching
// the given network identifier. Returns nil if no match.
func FilterAcceptsByNetwork(accepts []X402AcceptedPayment, network string) *X402AcceptedPayment {
	for i := range accepts {
		if accepts[i].Network == network {
			return &accepts[i]
		}
	}
	return nil
}

// doWithX402 wraps an HTTP request with x402 auto-sign-and-retry.
//
// On 402: decodes payment requirements, signs with the signer, and retries
// the original request with the PAYMENT-SIGNATURE header.
//
// bodyBytes is needed because http.Request.Body is consumed on first read
// and must be replayed on retry with bytes.NewReader.
func doWithX402(ctx context.Context, hc *http.Client, signer X402Signer, req *http.Request, bodyBytes []byte) (*http.Response, error) {
	// First attempt
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 402 {
		return res, nil
	}

	// ── Parse 402 ──
	requirements := ParsePaymentRequired(res.Header)
	_ = res.Body.Close()
	if requirements == nil {
		return nil, &PaymentRequiredError{
			APIError:            &APIError{StatusCode: 402, Message: "Payment required (unparseable requirements)"},
			PaymentRequirements: nil,
		}
	}

	// ── Find matching network ──
	accepted := FilterAcceptsByNetwork(requirements.Accepts, signer.Network())
	if accepted == nil {
		return nil, &PaymentRequiredError{
			APIError: &APIError{
				StatusCode: 402,
				Message:    fmt.Sprintf("No accepted payment for network %q", signer.Network()),
			},
			PaymentRequirements: requirements,
		}
	}

	// ── Sign payment ──
	signature, err := signer.Sign(ctx, requirements)
	if err != nil {
		return nil, fmt.Errorf("attago: x402 signing failed: %w", err)
	}

	// ── Retry with signature ──
	var retryBody io.Reader
	if bodyBytes != nil {
		retryBody = bytes.NewReader(bodyBytes)
	}

	retryReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), retryBody)
	if err != nil {
		return nil, fmt.Errorf("attago: build retry request: %w", err)
	}

	// Copy original headers
	for k, vv := range req.Header {
		for _, v := range vv {
			retryReq.Header.Add(k, v)
		}
	}
	retryReq.Header.Set("Payment-Signature", signature)

	retryRes, err := hc.Do(retryReq)
	if err != nil {
		return nil, err
	}

	if retryRes.StatusCode == 402 {
		_ = retryRes.Body.Close()
		return nil, &PaymentRequiredError{
			APIError: &APIError{
				StatusCode: 402,
				Message:    "Payment rejected after signing",
			},
			PaymentRequirements: requirements,
		}
	}

	return retryRes, nil
}
