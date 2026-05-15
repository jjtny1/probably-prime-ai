---
name: feedback-x402-sdk-v2-protocol
description: x402 SDK V2 protocol differences from V1 — critical for test construction
metadata:
  type: feedback
---

# x402 Go SDK V2 Protocol Notes

**Rule:** The x402 Go SDK (v0.0.0-20260513203758-9a718b002deb) uses the V2 protocol, which differs significantly from V1.

**Why:** Initial implementation assumed X-PAYMENT header (V1); SDK actually uses PAYMENT-SIGNATURE (V2). Body of 402 response is null, not JSON. This caused TestE2E tests to fail until corrected.

## Key V2 Differences

1. **Payment header to send**: `PAYMENT-SIGNATURE: base64(JSON(PaymentPayload))` NOT `X-PAYMENT`
2. **402 response body**: `null` (not JSON) — the SDK uses `json.Encoder` which writes null for nil
3. **402 response header**: `PAYMENT-REQUIRED: base64(JSON(PaymentRequired))` — decode this to get `{x402Version, error, resource, accepts}`
4. **PaymentPayload V2 shape**:
   ```json
   {"x402Version":2,"payload":{...},"accepted":{"scheme","network","asset","amount","payTo","maxTimeoutSeconds","extra"}}
   ```
5. **Route matching**: SDK matches on path ONLY (no query params). Use "GET /prime" not "GET /prime?bits=256"
6. **Facilitator endpoints**: GET /supported, POST /verify, POST /settle
7. **GetSupported populates asset cache**: Must be called (SyncFacilitatorOnStart=true) OR the fallback SupportedKind lacks asset info, causing FindMatchingRequirements to fail

## Default Facilitator URL
`https://x402.org/facilitator` — from `http.DefaultFacilitatorURL` constant in SDK

**How to apply:** Whenever writing x402 payment tests: use PAYMENT-SIGNATURE header, decode PAYMENT-REQUIRED header for assertions, build PaymentPayload with `accepted` field matching server's advertised requirements exactly.

## x402 Go Client API (for signing tests)

```go
import (
    x402core "github.com/x402-foundation/x402/go"
    x402http "github.com/x402-foundation/x402/go/http"
    evmexact "github.com/x402-foundation/x402/go/mechanisms/evm/exact/client"
    evmsigners "github.com/x402-foundation/x402/go/signers/evm"
)

signer, err := evmsigners.NewClientSignerFromPrivateKey(privateKeyHex) // accepts "0x..." or bare hex
x402Client := x402core.Newx402Client().Register("eip155:*", evmexact.NewExactEvmScheme(signer, nil))
httpClient := x402http.WrapHTTPClientWithPayment(&http.Client{}, x402http.Newx402HTTPClient(x402Client))
resp, err := httpClient.Do(req) // auto-handles 402 → sign EIP-3009 → retry
```

## Settlement Response Header
- Server sets `PAYMENT-RESPONSE` header (v2) — base64(JSON(SettleResponse))
- `SettleResponse.Transaction` = tx hash string "0x" + 64 hex chars
- `SettleResponse.Success` = true on success
- Decode manually: base64.StdEncoding.DecodeString → json.Unmarshal into x402core.SettleResponse
