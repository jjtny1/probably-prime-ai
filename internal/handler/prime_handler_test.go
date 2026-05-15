package handler_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jjtny1/probably-prime-ai/internal/handler"
	"github.com/stretchr/testify/require"
)

// realGenerator uses the actual prime package.
type realGenerator struct{}

func (g *realGenerator) Generate(bits int) (*big.Int, error) {
	// Import prime package via the generate function.
	// We use a small wrapper to avoid import cycles.
	return generatePrime(bits)
}

// errorGenerator always returns an error.
type errorGenerator struct{}

func (g *errorGenerator) Generate(_ int) (*big.Int, error) {
	return nil, errors.New("stub error")
}

// primeResponse is the JSON shape we expect from a successful response.
type primeResponse struct {
	Bits         int    `json:"bits"`
	Prime        string `json:"prime"`
	PrimeDecimal string `json:"prime_decimal"`
	Rounds       int    `json:"rounds"`
	GeneratedAt  string `json:"generated_at"`
}

// TestPrimeHandler_HappyPath_256: hit GET /prime/256, assert 200 + valid 256-bit prime.
func TestPrimeHandler_HappyPath_256(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&realGenerator{}))

	req := httptest.NewRequest(http.MethodGet, "/prime/256", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp primeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 256, resp.Bits)
	require.True(t, strings.HasPrefix(resp.Prime, "0x"), "prime should be hex-prefixed, got: %s", resp.Prime)

	// Decode and verify bit length
	n := new(big.Int)
	n.SetString(resp.Prime[2:], 16)
	require.Equal(t, 256, n.BitLen(), "decoded prime should be 256 bits")

	// Verify primality
	isPrime := n.ProbablyPrime(20)
	require.True(t, isPrime)
}

// TestPrimeHandler_HappyPath_Table: same for 512, 1024 (skip 2048 unless !testing.Short()).
func TestPrimeHandler_HappyPath_Table(t *testing.T) {
	bitsValues := []int{512, 1024}
	if !testing.Short() {
		bitsValues = append(bitsValues, 2048)
	}
	for _, bits := range bitsValues {
		bits := bits
		t.Run(fmt.Sprintf("bits=%d", bits), func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&realGenerator{}))

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/prime/%d", bits), nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			var resp primeResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Equal(t, bits, resp.Bits)
		})
	}
}

// TestPrimeHandler_MissingBits: GET /prime/ or GET /prime → 404 (no route registered for bare path).
func TestPrimeHandler_MissingBits(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&realGenerator{}))

	// A bare /prime path doesn't match the /{bits} pattern — returns 404.
	req := httptest.NewRequest(http.MethodGet, "/prime/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// /prime/ with trailing slash and no bits segment → 404 from mux
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestPrimeHandler_NonIntegerBits: GET /prime/abc → 400.
func TestPrimeHandler_NonIntegerBits(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&realGenerator{}))

	req := httptest.NewRequest(http.MethodGet, "/prime/abc", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "invalid bits", body["error"])
}

// TestPrimeHandler_DisallowedBits: GET /prime/128, /prime/384, /prime/4096 → 400.
func TestPrimeHandler_DisallowedBits(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&realGenerator{}))

	for _, bits := range []string{"128", "384", "4096"} {
		bits := bits
		t.Run(bits, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/prime/"+bits, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code)
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, "invalid bits", body["error"])
		})
	}
}

// TestPrimeHandler_GenerationFailure: inject a prime-generator stub that always errors.
// Assert 500 with {"error":"prime generation failed"}.
func TestPrimeHandler_GenerationFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&errorGenerator{}))

	req := httptest.NewRequest(http.MethodGet, "/prime/256", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "prime generation failed", body["error"])
}

// TestPrimeHandler_ResponseContentType: success path response has Content-Type: application/json.
func TestPrimeHandler_ResponseContentType(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&realGenerator{}))

	req := httptest.NewRequest(http.MethodGet, "/prime/256", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	ct := w.Header().Get("Content-Type")
	require.True(t, strings.HasPrefix(ct, "application/json"), "expected application/json, got: %s", ct)
}

// TestPrimeHandler_GeneratedAtIsRecent: response generated_at parses as RFC 3339
// and is within 5s of time.Now().
func TestPrimeHandler_GeneratedAtIsRecent(t *testing.T) {
	before := time.Now()
	mux := http.NewServeMux()
	mux.Handle("GET /prime/{bits}", handler.NewPrimeHandler(&realGenerator{}))

	req := httptest.NewRequest(http.MethodGet, "/prime/256", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	after := time.Now()

	require.Equal(t, http.StatusOK, w.Code)
	var resp primeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	ts, err := time.Parse(time.RFC3339, resp.GeneratedAt)
	require.NoError(t, err)
	require.True(t, ts.After(before.Add(-time.Second)), "generated_at too early")
	require.True(t, ts.Before(after.Add(5*time.Second)), "generated_at too late")
}
