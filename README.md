# attago-go-sdk

[![CI](https://github.com/AttaGo/attago-go-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/AttaGo/attago-go-sdk/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/AttaGo/attago-go-sdk.svg)](https://pkg.go.dev/github.com/AttaGo/attago-go-sdk)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Go SDK for the [AttaGo](https://attago.bid) crypto trading dashboard API.

Zero dependencies. Go 1.23+ required.

## Installation

```bash
go get github.com/AttaGo/attago-go-sdk
```

## Quick Start

### API Key (scripts, bots, CI)

```go
package main

import (
	"context"
	"fmt"
	"log"

	attago "github.com/AttaGo/attago-go-sdk"
)

func main() {
	client := attago.NewClient(attago.WithAPIKey("ak_live_..."))

	score, err := client.Agent.GetScore(context.Background(), "BTC")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s: %s (score %d, confidence %.0f%%)\n",
		score.Token,
		score.Composite.Signal,
		score.Composite.Score,
		score.Composite.Confidence*100,
	)
}
```

### x402 Signer (anonymous wallet agents)

```go
client := attago.NewClient(attago.WithSigner(mySigner))

// 402 responses are auto-signed and retried
score, err := client.Agent.GetScore(ctx, "BTC")
```

The `Signer` interface:

```go
type X402Signer interface {
	Address() string
	Network() string // e.g. "eip155:8453" (Base)
	Sign(ctx context.Context, reqs *X402PaymentRequirements) (string, error)
}
```

### Cognito (account management)

```go
client := attago.NewClient(attago.WithCognito(
	"you@example.com",
	"your-password",
	"your-cognito-client-id",
))

// Manage subscriptions
subs, _ := client.Subscriptions.List(ctx)
client.Subscriptions.Create(ctx, attago.CreateSubscriptionInput{
	TokenID:    "BTC",
	Label:      "BTC spot bullish",
	Groups:     [][]attago.SubscriptionCondition{
		{{MetricName: "spot_score", ThresholdOp: "gt", ThresholdVal: 5}},
	},
})

// Check billing
status, _ := client.Payments.Status(ctx)
```

## API Reference

### Agent

| Method | Description | Auth |
|--------|-------------|------|
| `client.Agent.GetScore(ctx, symbol)` | Single asset Go/No-Go score | API key or x402 |
| `client.Agent.GetData(ctx, symbols...)` | Full market data for all or filtered assets | API key or x402 |

### Data

| Method | Description | Auth |
|--------|-------------|------|
| `client.Data.GetLatest(ctx)` | Latest tiered data snapshot | API key or JWT |
| `client.Data.GetTokenData(ctx, token)` | Single token data | API key or JWT |
| `client.Data.GetDataPush(ctx, requestID)` | 72h data-push snapshot | None |

### Subscriptions (alerts)

| Method | Description | Auth |
|--------|-------------|------|
| `client.Subscriptions.Catalog(ctx)` | Available tokens, metrics, tier limits | API key or JWT |
| `client.Subscriptions.List(ctx)` | User's active subscriptions | API key or JWT |
| `client.Subscriptions.Create(ctx, input)` | Create a new alert | API key or JWT |
| `client.Subscriptions.Update(ctx, id, input)` | Update an alert | API key or JWT |
| `client.Subscriptions.Delete(ctx, id)` | Delete an alert | API key or JWT |

### Payments

| Method | Description | Auth |
|--------|-------------|------|
| `client.Payments.Subscribe(ctx, input)` | Purchase a tier (x402 in live mode) | API key or JWT |
| `client.Payments.Status(ctx)` | Current tier, expiry, push usage | API key or JWT |
| `client.Payments.UpgradeQuote(ctx, tier, cycle)` | Pro-rated upgrade quote | API key or JWT |

### Wallets

| Method | Description | Auth |
|--------|-------------|------|
| `client.Wallets.Register(ctx, input)` | Verify + register a wallet | API key or JWT |
| `client.Wallets.List(ctx)` | List verified wallets | API key or JWT |
| `client.Wallets.Remove(ctx, address)` | Remove a wallet | API key or JWT |

### Webhooks

| Method | Description | Auth |
|--------|-------------|------|
| `client.Webhooks.Create(ctx, url)` | Register a webhook (Pro+) | API key or JWT |
| `client.Webhooks.List(ctx)` | List webhooks (secrets stripped) | API key or JWT |
| `client.Webhooks.Delete(ctx, id)` | Remove a webhook | API key or JWT |
| `client.Webhooks.SendTest(ctx, opts)` | SDK-side test delivery (local) | None |
| `client.Webhooks.SendServerTest(ctx, id)` | Server-side test delivery | API key or JWT |

### MCP (Model Context Protocol)

| Method | Description | Auth |
|--------|-------------|------|
| `client.MCP.Initialize(ctx)` | Negotiate protocol version | None |
| `client.MCP.ListTools(ctx)` | Discover tools + pricing | None |
| `client.MCP.CallTool(ctx, name, args)` | Execute a tool | x402 for paid tools |
| `client.MCP.Ping(ctx)` | Health check | None |

### Other

| Method | Description | Auth |
|--------|-------------|------|
| `client.APIKeys.Create(ctx, name)` | Create API key (JWT only) | JWT |
| `client.APIKeys.List(ctx)` | List API keys | API key or JWT |
| `client.APIKeys.Revoke(ctx, keyID)` | Revoke an API key | API key or JWT |
| `client.Bundles.List(ctx)` | List bundles + catalog | API key or JWT |
| `client.Bundles.Purchase(ctx, input)` | Buy data-push credits | API key or JWT |
| `client.Push.List(ctx)` | List push subscriptions | API key or JWT |
| `client.Push.Create(ctx, input)` | Register push subscription | API key or JWT |
| `client.Push.Delete(ctx, id)` | Remove push subscription | API key or JWT |
| `client.Redeem.Redeem(ctx, code)` | Activate a redemption code | API key or JWT |

## Webhook Listener

Built-in HTTP server for receiving webhooks with HMAC-SHA256 verification:

```go
listener := attago.NewWebhookListener(attago.WebhookListenerConfig{
	Secret: "wh_secret_from_creation",
	Port:   4000,
})

listener.OnAlert(func(p attago.WebhookPayload) {
	fmt.Printf("%s %s\n", p.Alert.Token, p.Alert.State)
})

listener.OnTest(func(p attago.WebhookPayload) {
	fmt.Println("Test delivery received!")
})

listener.OnError(func(err error) {
	fmt.Fprintf(os.Stderr, "Listener error: %v\n", err)
})

if err := listener.Start(); err != nil {
	log.Fatal(err)
}
defer listener.Stop(context.Background())

fmt.Printf("Listening on %s\n", listener.Addr())
```

## Signature Verification

For custom webhook receivers (without the built-in listener):

```go
import attago "github.com/AttaGo/attago-go-sdk"

isValid := attago.VerifySignature(
	rawBody,        // []byte — raw request body
	webhookSecret,  // string — from webhook creation
	req.Header.Get("X-AttaGo-Signature"),
)
```

## Error Handling

All errors support `errors.As()` for typed checking:

```go
import "errors"

score, err := client.Agent.GetScore(ctx, "BTC")
if err != nil {
	var payErr *attago.PaymentRequiredError
	var rateErr *attago.RateLimitError
	var apiErr *attago.APIError

	switch {
	case errors.As(err, &payErr):
		// 402 — payment needed (x402 signer handles this automatically)
		fmt.Println(payErr.PaymentRequirements)

	case errors.As(err, &rateErr):
		// 429 — wait and retry
		fmt.Printf("retry after %d seconds\n", rateErr.RetryAfter)

	case errors.As(err, &apiErr):
		// Other HTTP errors
		fmt.Printf("%d: %s\n", apiErr.StatusCode, apiErr.Message)
	}
}
```

## Testing

```bash
go test -v -count=1 ./...                       # Unit tests
go test -v -count=1 -race ./...                  # Unit tests + race detector

# Conformance (needs env vars + attago-spec clone)
ATTAGO_BASE_URL=https://dev.attago.bid \
ATTAGO_API_KEY=ak_live_... \
ATTAGO_SPEC_DIR=../attago-spec \
go test -v -tags conformance -run Conformance ./...
```

## Requirements

- Go 1.23+
- Zero runtime dependencies (stdlib only)

## License

MIT
