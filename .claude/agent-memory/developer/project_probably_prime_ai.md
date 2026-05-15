---
name: project-probably-prime-ai
description: Key conventions, build commands, and gotchas for the probably_prime_ai Go service
metadata:
  type: project
---

# probably_prime_ai project notes

**Why:** x402-monetized HTTP prime-generation API. Go module `github.com/jjtny1/probably-prime-ai` (renamed from `github.com/probably-prime-ai/probably_prime_ai` on 2026-05-14 to match https://github.com/jjtny1/probably-prime-ai).

## Build & Test Commands

- Build: `go build ./cmd/server/...`
- Test: `go test ./... -timeout 120s`
- Test with race: `go test ./... -race -timeout 120s`
- MR flake check: `go test ./... -count=10 -run TestIsProbablePrime -timeout 180s`
- Vet: `go vet ./...`
- Format check: `gofmt -l .`
- Live e2e: `go test -tags live_e2e ./test/e2e/... -v -run TestLive -timeout 300s`
- Slow primes: `go test -tags slow ./internal/prime/... -v`

## Key Pinned Version
- x402 SDK: `github.com/x402-foundation/x402/go v0.0.0-20260513203758-9a718b002deb`
- testify: `v1.11.1`

## Package Layout
```
cmd/server/         entry point
internal/config/    env var loading (Load() returns *Config, error)
internal/pricing/   Tier(bits) → (price, ok); AllowedBits()
internal/prime/     Generate(bits, rnd, maxRetries); IsProbablePrime(n, rnd)
internal/handler/   NewPrimeHandler(PrimeGenerator) http.HandlerFunc
internal/x402middleware/ BuildRoutesConfig(cfg); Wrap(handler, cfg, facilitatorClient)
test/e2e/           mocked_test.go (!live_e2e); live_test.go (live_e2e tag)
test/e2e/testhelpers/ FacilitatorStub with nonce-replay protection
```

## Critical Conventions
- `crypto/rand` only in internal/prime — math/rand is forbidden
- `IsProbablePrime(n, rnd io.Reader)` — rnd is injectable for tests
- `Generate(bits, rnd, maxRetries)` — all three args explicit
- API is PATH-based: `GET /prime/{bits}` (NOT query params). Four registered routes: `/prime/256`, `/prime/512`, `/prime/1024`, `/prime/2048`, each with tier-specific price.
- Handler reads `r.PathValue("bits")` (Go 1.22+ ServeMux pattern), validates via `pricing.Tier()`.
- `BuildRoutesConfig` emits 4 keys, one per `pricing.AllowedBits()` entry, with per-tier price strings.
- `deterministicWitnesses(n)` returns nil for n >= boundary (use random witnesses)
- Inner mux registers `"GET /prime/{bits}"` pattern; outer mux delegates `/` to the x402-wrapped inner mux.

**Why:** A previous implementation used a single `GET /prime` route with `?bits=N` query param. The SDK matches routes by path only (not query params), so all bit sizes got the same price ($0.001). Switching to path-based routes gives 4 distinct RoutesConfig keys and correct per-tier on-chain charges.

**How to apply:** Follow these conventions when adding features to any of these packages.
