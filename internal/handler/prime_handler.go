// Package handler implements the HTTP handler for the /prime/{bits} endpoint.
// It is independent of payment logic — x402 middleware is applied at a higher level.
package handler

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/jjtny1/probably-prime-ai/internal/pricing"
)

// PrimeGenerator is the interface the handler uses to produce primes.
// The production implementation wraps prime.Generate; tests may inject a stub.
type PrimeGenerator interface {
	Generate(bits int) (*big.Int, error)
}

// primeResponse is the JSON response body for a successful prime request.
type primeResponse struct {
	Bits         int    `json:"bits"`
	Prime        string `json:"prime"`
	PrimeDecimal string `json:"prime_decimal"`
	Rounds       int    `json:"rounds"`
	GeneratedAt  string `json:"generated_at"`
}

// errorResponse is the JSON response body for error cases.
type errorResponse struct {
	Error   string `json:"error"`
	Allowed []int  `json:"allowed,omitempty"`
}

// NewPrimeHandler returns an http.HandlerFunc that handles GET /prime/{bits}.
// The bits path segment is parsed and validated against allowed pricing tiers.
// Payment enforcement is handled by the x402 middleware wrapping this handler.
func NewPrimeHandler(gen PrimeGenerator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the {bits} path value from the request (Go 1.22+ ServeMux pattern).
		bitsStr := r.PathValue("bits")

		// Validate bits parameter — must be a parseable integer.
		bits, err := strconv.Atoi(bitsStr)
		if err != nil || bitsStr == "" {
			writeError(w, http.StatusBadRequest, "invalid bits", pricing.AllowedBits())
			return
		}

		// Validate that bits is one of the allowed pricing tiers.
		if _, ok := pricing.Tier(bits); !ok {
			writeError(w, http.StatusBadRequest, "invalid bits", pricing.AllowedBits())
			return
		}

		// Generate prime.
		p, err := gen.Generate(bits)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "prime generation failed", nil)
			return
		}

		resp := primeResponse{
			Bits:         bits,
			Prime:        fmt.Sprintf("0x%X", p),
			PrimeDecimal: p.String(),
			Rounds:       roundsForBits(bits),
			GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// writeError writes a JSON error response with the given status code.
func writeError(w http.ResponseWriter, status int, msg string, allowed []int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := errorResponse{Error: msg, Allowed: allowed}
	_ = json.NewEncoder(w).Encode(resp)
}

// roundsForBits returns the Miller-Rabin round count used for the given bit size.
// This mirrors the value from internal/prime/witnesses.go.
func roundsForBits(bits int) int {
	switch {
	case bits >= 2048:
		return 27
	case bits >= 1536:
		return 33
	case bits >= 1024:
		return 40
	case bits >= 512:
		return 56
	default:
		return 64
	}
}
