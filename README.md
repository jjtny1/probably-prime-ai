# probably_prime_ai

An x402-monetized HTTP API for generating cryptographically-secure probable primes.

## Overview

`probably_prime_ai` generates probable primes of caller-specified bit lengths (256/512/1024/2048 bits) using Miller-Rabin primality testing with a false-positive probability ≤ 2⁻¹²⁸. Each API call is gated behind an [x402](https://x402.org) payment — callers pay USDC on Base (Sepolia by default) before receiving the prime. Designed for machine-to-machine use by AI agents and automated systems.

```
Agent ──GET /prime/1024──► Server ──► HTTP 402 + PAYMENT-REQUIRED header
Agent ──GET /prime/1024──► Server    (with PAYMENT-SIGNATURE header)
       ◄──HTTP 200 + prime JSON──
```

## Architecture

```
cmd/server/         ← entry point: env → handler → middleware → ListenAndServe
internal/config/    ← env var parsing and validation
internal/pricing/   ← bits → USD price mapping
internal/prime/     ← Miller-Rabin primality + prime generation (crypto/rand only)
internal/handler/   ← HTTP handler for /prime/{bits} (no payment logic)
internal/x402middleware/ ← x402 SDK adapter
test/e2e/           ← mocked and live end-to-end tests
```

## Quickstart

### Prerequisites

- Go 1.24+
- A Base Sepolia wallet with ETH (for gas) and USDC (for payments)
- An Ethereum address to receive payments (`X402_PAY_TO`)

### Build and Run

```bash
git clone https://github.com/jjtny1/probably-prime-ai
cd probably_prime_ai

# Set required environment variable
export X402_PAY_TO=0x<your-ethereum-address>

# Optional: override defaults
export X402_NETWORK=base-sepolia          # or "base" for mainnet
export PORT=4021
export X402_FACILITATOR_URL=https://x402.org/facilitator

# Build and run
go build -o probably_prime_ai ./cmd/server
./probably_prime_ai
```

The server starts on `http://localhost:4021`.

### Quick Test (without paying)

```bash
# Should return HTTP 402 with payment requirements in PAYMENT-REQUIRED header
curl -v http://localhost:4021/prime/256

# Health check (no payment required)
curl http://localhost:4021/health

# Pricing info (no payment required)
curl http://localhost:4021/pricing
```

## Configuration

All configuration is via environment variables.

| Variable | Default | Required | Description |
|---|---|---|---|
| `PORT` | `4021` | no | TCP port to listen on (1–65535) |
| `X402_NETWORK` | `base-sepolia` | no | `base-sepolia` or `base` |
| `X402_PAY_TO` | — | **yes** | 0x-prefixed Ethereum address to receive USDC |
| `X402_FACILITATOR_URL` | `https://x402.org/facilitator` | no | x402 facilitator endpoint |
| `X402_SCHEME` | `exact` | no | x402 payment scheme |
| `PRIME_MAX_GENERATION_RETRIES` | `10000` | no | Max candidate draws per prime generation |
| `LIVE_E2E` | unset | live tests only | Set to `1` to enable live e2e test execution |
| `LIVE_E2E_PRIVATE_KEY` | unset | live tests only | 0x-prefixed private key for test wallet |
| `LIVE_E2E_RPC_URL` | unset | live tests only | Base Sepolia RPC URL |

## API Reference

### `GET /prime/{bits}`

**Requires payment via x402.** Generates a probable prime of the specified bit length.

The `{bits}` path segment must be one of: `256`, `512`, `1024`, `2048`.

Each tier has a distinct price registered with the x402 middleware, so the on-chain
USDC charge matches the selected bit size exactly.

**Request**: No body. Bit size encoded in the URL path.

**Examples**:
- `GET /prime/256` — 256-bit prime, $0.001 USDC
- `GET /prime/512` — 512-bit prime, $0.003 USDC
- `GET /prime/1024` — 1024-bit prime, $0.01 USDC
- `GET /prime/2048` — 2048-bit prime, $0.05 USDC

**Unpaid response (HTTP 402)**:
- Status: `402 Payment Required`
- Header `PAYMENT-REQUIRED`: base64(JSON) containing payment requirements
```json
{
  "x402Version": 2,
  "error": "Payment required",
  "resource": {...},
  "accepts": [{
    "scheme": "exact",
    "network": "eip155:84532",
    "asset": "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
    "amount": "1000",
    "payTo": "0x...",
    "maxTimeoutSeconds": 60
  }]
}
```

**Paid response (HTTP 200)**:
```json
{
  "bits": 1024,
  "prime": "0x...",
  "prime_decimal": "1234...",
  "rounds": 40,
  "generated_at": "2026-05-14T12:34:56Z"
}
```

**Error responses**:
- `400 Bad Request`: `{"error":"invalid bits","allowed":[256,512,1024,2048]}`
- `500 Internal Server Error`: `{"error":"prime generation failed"}`

### `GET /health`

No payment required. Returns `{"status":"ok"}` with HTTP 200.

### `GET /pricing`

No payment required. Returns the pricing tier table.

```json
{
  "tiers": [
    {"bits": 256, "price_usd": "0.001"},
    {"bits": 512, "price_usd": "0.003"},
    {"bits": 1024, "price_usd": "0.01"},
    {"bits": 2048, "price_usd": "0.05"}
  ],
  "network": "base-sepolia",
  "pay_to": "0x..."
}
```

## Pricing Tiers

| Bits | Price (USDC) | Expected Latency |
|---|---|---|
| 256  | $0.001 | < 10ms |
| 512  | $0.003 | < 100ms |
| 1024 | $0.01  | < 1s |
| 2048 | $0.05  | 1–10s |

All prices are USDC on Base. The `amount` field in the 402 response is in USDC atomic units (6 decimals), so $0.001 = 1000 atomic units.

Each bit size has a separate route registered with the x402 middleware
(`GET /prime/256`, `GET /prime/512`, `GET /prime/1024`, `GET /prime/2048`).
This ensures the on-chain charge exactly matches the requested tier.

## Primality Algorithm

Prime generation uses:
1. `crypto/rand` for candidate generation (never `math/rand`)
2. Small-prime trial division (primes up to 1000)
3. Miller-Rabin primality test:
   - Deterministic witness set {2,3,5,7,11,13,17,19,23,29,31,37} for n < 3,317,044,064,679,887,385,961,981 (Sorenson & Webster 2017)
   - Randomized witnesses for larger n, with round count per FIPS 186-4 Table C.1

False-positive probability: ≤ 2⁻¹²⁸ for all supported bit sizes.

## Live E2E Test Runbook

### Prerequisites

1. Generate or reuse a Base Sepolia test wallet
2. Fund it with Base Sepolia ETH ([Coinbase Faucet](https://coinbase.com/faucets) or [Alchemy Faucet](https://faucets.alchemy.com))
3. Fund it with at least 0.05 Base Sepolia USDC (see [x402 docs](https://x402.org/docs) for faucet links)

### Run the live tests

```bash
export LIVE_E2E=1
export LIVE_E2E_PRIVATE_KEY=0x<64-hex-private-key>
export LIVE_E2E_RPC_URL=https://sepolia.base.org
export X402_PAY_TO=0x<your-test-wallet-address>

go test -tags live_e2e ./test/e2e/... -v -run TestLive -timeout 300s
```

**Cost**: Each live test run costs approximately $0.001–$0.05 in testnet USDC (from a faucet). Testnet USDC has no real-world value.

**Expected duration**: 10–60 seconds per test (Base Sepolia block times).

## Security

- All randomness uses `crypto/rand` — `math/rand` is never used in prime generation
- Pay-to address is never logged in full (last 6 characters only)
- No PII is collected; x402 payment is the authentication mechanism
- Replay protection: EIP-3009 nonce mechanism via the x402 facilitator
- No secrets or API keys stored; configuration is env-only

## Limitations (Out of Scope)

- No persistence (primes are not stored)
- No rate limiting beyond the x402 payment cost barrier
- No multi-asset support (USDC on Base only)
- No batch endpoint
- No Prometheus/OpenTelemetry metrics (logs only)
- No RSA key pair generation (single primes only)
