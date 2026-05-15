---
name: project-probably-prime-ai-x402
description: probably_prime_ai is a Go HTTP service that sells Miller-Rabin-verified probable primes monetized via the x402 protocol on Base
metadata:
  type: project
---

`probably_prime_ai` is a fresh Go project. Its single purpose is an HTTP API that returns probable primes of caller-specified bit length (256/512/1024/2048), monetized per-request via the x402 protocol with USDC on Base (Sepolia default, mainnet optional).

**Why:** Built as a machine-to-machine product for AI agents — no human auth, x402 payment is the access control. Confirmed user-locked decisions: Go language, stdlib `net/http` + x402 SDK middleware, tiered pricing ($0.001/$0.003/$0.01/$0.05), network configurable via env, tests at four depths including a live Base Sepolia e2e gated by build tag + env.

**How to apply:** When the user asks for new features or changes here, the existing plan at `.claude/docs/2026-05-14-x402-prime-api.md` is the source of truth for architecture. Future plans should respect: TDD enforcement (see [[feedback-tdd-mandatory]]), `crypto/rand` only in `internal/prime/`, no rate limiting / no DB / no auth beyond x402 (explicit non-goals), and the existing package boundaries (`cmd/server`, `internal/prime`, `internal/pricing`, `internal/config`, `internal/handler`, `internal/x402middleware`, `test/e2e`).
