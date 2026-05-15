// Package main is the entry point for the probably_prime_ai server.
// It loads configuration, wires dependencies, and starts the HTTP server.
package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"

	x402http "github.com/x402-foundation/x402/go/http"

	"github.com/jjtny1/probably-prime-ai/internal/config"
	"github.com/jjtny1/probably-prime-ai/internal/handler"
	"github.com/jjtny1/probably-prime-ai/internal/pricing"
	"github.com/jjtny1/probably-prime-ai/internal/prime"
	"github.com/jjtny1/probably-prime-ai/internal/x402middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Log startup info (PayTo with last 6 chars only for security).
	payToPartial := cfg.PayTo
	if len(payToPartial) > 6 {
		payToPartial = "..." + payToPartial[len(payToPartial)-6:]
	}
	slog.Info("starting server",
		"port", cfg.Port,
		"network", cfg.Network,
		"pay_to_partial", payToPartial,
		"facilitator_url", cfg.FacilitatorURL,
	)

	// Wire prime generator.
	gen := &primeGenerator{
		maxRetries: cfg.PrimeMaxGenerationRetries,
	}

	// Build handlers.
	primeH := handler.NewPrimeHandler(gen)
	healthH := healthHandler()
	pricingH := pricingHandler(cfg)

	// Build facilitator client.
	facilitatorClient := x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{
		URL: cfg.FacilitatorURL,
	})

	// Build inner mux with per-tier path routes (GET /prime/{bits}).
	// Each path maps to the same handler, which extracts the {bits} path value.
	innerMux := http.NewServeMux()
	innerMux.Handle("GET /prime/{bits}", primeH)

	// Wrap with x402 middleware. The middleware's RoutesConfig has one entry
	// per allowed bit size (GET /prime/256, GET /prime/512, etc.) with the
	// tier-specific price, so the on-chain charge is correct per request.
	protectedHandler := x402middleware.Wrap(innerMux, cfg, facilitatorClient)

	// Outer mux handles unprotected routes and routes all other traffic to the protected handler.
	mux := http.NewServeMux()
	mux.Handle("/health", healthH)
	mux.Handle("/pricing", pricingH)
	mux.Handle("/", protectedHandler)

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// primeGenerator wraps prime.Generate and implements handler.PrimeGenerator.
type primeGenerator struct {
	maxRetries int
}

// Generate implements handler.PrimeGenerator.
func (g *primeGenerator) Generate(bits int) (*big.Int, error) {
	return prime.Generate(bits, rand.Reader, g.maxRetries)
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func pricingHandler(cfg *config.Config) http.HandlerFunc {
	type tier struct {
		Bits     int    `json:"bits"`
		PriceUSD string `json:"price_usd"`
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		allowed := pricing.AllowedBits()
		tiers := make([]tier, len(allowed))
		for i, bits := range allowed {
			price, _ := pricing.Tier(bits)
			// Strip the leading '$' so the response contains "0.001" not "$0.001".
			priceVal := price
			if len(priceVal) > 0 && priceVal[0] == '$' {
				priceVal = priceVal[1:]
			}
			tiers[i] = tier{
				Bits:     bits,
				PriceUSD: priceVal,
			}
		}
		resp := map[string]interface{}{
			"tiers":   tiers,
			"network": cfg.Network,
			"pay_to":  cfg.PayTo,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
