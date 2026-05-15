---
name: reference-x402-go-sdk
description: x402 protocol Go SDK lives at github.com/x402-foundation/x402/go (not coinbase/x402); key import paths, network identifiers, header conventions
metadata:
  type: reference
---

The x402 protocol's canonical Go SDK module path is **`github.com/x402-foundation/x402/go`**, not `github.com/coinbase/x402` — the Coinbase-named repo is the canonical source tree but Go consumers import from the foundation path (per the repo's `go.mod`).

**Key import paths used in plans:**
- `github.com/x402-foundation/x402/go` — core types and client
- `github.com/x402-foundation/x402/go/http` — `NewHTTPFacilitatorClient`, `RoutesConfig`, `PaymentOptions`
- `github.com/x402-foundation/x402/go/http/nethttp` — `X402Payment(Config)` middleware for stdlib `net/http`
- `github.com/x402-foundation/x402/go/mechanisms/evm/exact/server` — `NewExactEvmScheme()` for Base/Ethereum

**Network identifiers** (per x402 spec):
- Base mainnet: `eip155:8453` (network string `"base"`)
- Base Sepolia: `eip155:84532` (network string `"base-sepolia"`)
- USDC mainnet: `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913`
- USDC Sepolia: `0x036CbD53842c5426634e7929541eC2318f3dCF7e`

**Protocol headers:**
- `X-PAYMENT` (request): base64-encoded `PaymentPayload` JSON with fields `x402Version`, `scheme`, `network`, `payload{signature, authorization}`
- `X-PAYMENT-RESPONSE` (response): base64-encoded `SettlementResponse` with `success`, `transaction`, `network`, `payer` (or `errorReason` on failure)

**402 response body shape:** `{ "x402Version": 1, "error": "...", "accepts": [PaymentRequirements...] }`.

**PaymentRequirements fields:** `scheme`, `network`, `maxAmountRequired`, `asset`, `payTo`, `resource`, `description`, `mimeType` (optional), `outputSchema` (optional), `maxTimeoutSeconds`, `extra` (optional).

**How to apply:** Use these import paths and identifiers verbatim when planning x402 integrations in Go. Before recommending a specific SDK function name, verify it still exists in the installed version — the SDK was at Go 1.24 and used the `x402-foundation` path as of 2026-05-14, but APIs may evolve. Related: [[project-probably-prime-ai-x402]].
