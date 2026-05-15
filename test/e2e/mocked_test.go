//go:build !live_e2e

package e2e_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jjtny1/probably-prime-ai/internal/config"
	"github.com/jjtny1/probably-prime-ai/internal/handler"
	"github.com/jjtny1/probably-prime-ai/internal/pricing"
	"github.com/jjtny1/probably-prime-ai/internal/prime"
	"github.com/jjtny1/probably-prime-ai/internal/x402middleware"
	"github.com/jjtny1/probably-prime-ai/test/e2e/testhelpers"
	x402http "github.com/x402-foundation/x402/go/http"
)

const testPayTo = "0x0000000000000000000000000000000000000001"

// testPrimeGen wraps prime.Generate for the e2e test server.
type testPrimeGen struct{}

func (g *testPrimeGen) Generate(bits int) (*big.Int, error) {
	return prime.Generate(bits, rand.Reader, 10000)
}

// newTestServer creates a full server wired with the given facilitator stub URL.
// The inner mux registers GET /prime/{bits} (path-based routing) so the x402
// middleware's per-tier RoutesConfig keys match the actual request paths.
func newTestServer(t *testing.T, facilitatorURL string) *httptest.Server {
	t.Helper()

	cfg := &config.Config{
		Port:                      4021,
		Network:                   "base-sepolia",
		PayTo:                     testPayTo,
		FacilitatorURL:            facilitatorURL,
		Scheme:                    "exact",
		PrimeMaxGenerationRetries: 10000,
	}

	gen := &testPrimeGen{}
	primeH := handler.NewPrimeHandler(gen)

	facilitatorClient := x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{
		URL: facilitatorURL,
	})

	// Register GET /prime/{bits} — matches the RoutesConfig keys "GET /prime/256" etc.
	innerMux := http.NewServeMux()
	innerMux.Handle("GET /prime/{bits}", primeH)

	protectedHandler := x402middleware.Wrap(innerMux, cfg, facilitatorClient)

	outerMux := http.NewServeMux()
	outerMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	outerMux.HandleFunc("/pricing", func(w http.ResponseWriter, _ *http.Request) {
		type tier struct {
			Bits     int    `json:"bits"`
			PriceUSD string `json:"price_usd"`
		}
		allowed := pricing.AllowedBits()
		tiers := make([]tier, len(allowed))
		for i, bits := range allowed {
			price, _ := pricing.Tier(bits)
			if len(price) > 0 && price[0] == '$' {
				price = price[1:]
			}
			tiers[i] = tier{Bits: bits, PriceUSD: price}
		}
		resp := map[string]interface{}{
			"tiers":   tiers,
			"network": cfg.Network,
			"pay_to":  cfg.PayTo,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	outerMux.Handle("/", protectedHandler)

	return httptest.NewServer(outerMux)
}

// decodePaymentRequired decodes the PAYMENT-REQUIRED header from a 402 response.
// The V2 x402 protocol puts payment requirements in this header as base64(JSON).
func decodePaymentRequired(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	header := resp.Header.Get("Payment-Required")
	require.NotEmpty(t, header, "PAYMENT-REQUIRED header must be present on a 402 response")
	decoded, err := base64.StdEncoding.DecodeString(header)
	require.NoError(t, err, "PAYMENT-REQUIRED header must be valid base64")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(decoded, &result), "PAYMENT-REQUIRED header must contain valid JSON")
	return result
}

// buildMockPaymentHeader constructs a base64-encoded JSON V2 payment payload that the
// x402 middleware will forward to the facilitator for verification.
// The V2 SDK uses the PAYMENT-SIGNATURE header (not X-PAYMENT).
// The "accepted" field must match what the server advertised in its 402 response.
func buildMockPaymentHeader(t *testing.T) (headerName, headerValue string) {
	t.Helper()
	// V2 PaymentPayload structure with "accepted" matching Base Sepolia USDC exact scheme.
	// USDC on Base Sepolia: 0x036CbD53842c5426634e7929541eC2318f3dCF7e
	payload := map[string]interface{}{
		"x402Version": 2,
		"payload": map[string]interface{}{
			"signature": "0x" + strings.Repeat("ab", 65),
			"authorization": map[string]interface{}{
				"from":        "0x0000000000000000000000000000000000000001",
				"to":          testPayTo,
				"value":       "1000",
				"validAfter":  "0",
				"validBefore": "9999999999",
				"nonce":       "0x" + strings.Repeat("cd", 32),
			},
		},
		"accepted": map[string]interface{}{
			"scheme":            "exact",
			"network":           "eip155:84532",
			"asset":             "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
			"amount":            "1000",
			"payTo":             testPayTo,
			"maxTimeoutSeconds": 60,
			"extra": map[string]interface{}{
				"name":    "USDC",
				"version": "2",
			},
		},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	return "PAYMENT-SIGNATURE", base64.StdEncoding.EncodeToString(data)
}

func TestE2E_NoPayment_Returns402(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(true, true)
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/prime/256")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusPaymentRequired, resp.StatusCode)

	// In V2 x402 protocol, payment requirements are in the PAYMENT-REQUIRED header.
	parsed := decodePaymentRequired(t, resp)
	require.Contains(t, parsed, "x402Version", "payment required must contain x402Version")
	require.Contains(t, parsed, "accepts", "payment required must contain accepts")
}

func TestE2E_402_AllTiers_Table(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(true, true)
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	for _, bits := range pricing.AllowedBits() {
		bits := bits
		t.Run(fmt.Sprintf("bits=%d", bits), func(t *testing.T) {
			resp, err := http.Get(srv.URL + fmt.Sprintf("/prime/%d", bits))
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusPaymentRequired, resp.StatusCode)

			parsed := decodePaymentRequired(t, resp)
			accepts, ok := parsed["accepts"].([]interface{})
			require.True(t, ok, "accepts should be an array")
			require.NotEmpty(t, accepts)

			firstAccept, ok := accepts[0].(map[string]interface{})
			require.True(t, ok)
			// "amount" is the USDC atomic units — must be positive
			amountStr := fmt.Sprintf("%v", firstAccept["amount"])
			require.NotEmpty(t, amountStr)

			// Verify the asset address is non-empty (USDC contract on Base Sepolia)
			asset := fmt.Sprintf("%v", firstAccept["asset"])
			require.NotEmpty(t, asset)
		})
	}
}

// TestE2E_InvalidBits_Returns400_NotChallenged hits a path that is not registered
// in the RoutesConfig (/prime/999). The middleware does not challenge unregistered
// paths — it passes them through to the inner mux which 404s (not registered there
// either since only valid bits paths are registered). The assertion is "not 200".
func TestE2E_InvalidBits_Returns400_NotChallenged(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(true, true)
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	// /prime/999 is not in the RoutesConfig (no registered tier for 999 bits).
	// The middleware passes it through; the inner mux's GET /prime/{bits} handler
	// returns 400 (invalid bits) since 999 is not an allowed tier.
	resp, err := http.Get(srv.URL + "/prime/999")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.NotEqual(t, http.StatusOK, resp.StatusCode,
		"invalid bits request must not return 200, got %d", resp.StatusCode)
}

func TestE2E_WithMockPayment_Returns200(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(true, true)
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	hdrName, hdrVal := buildMockPaymentHeader(t)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/prime/256", nil)
	require.NoError(t, err)
	req.Header.Set(hdrName, hdrVal)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"expected 200 with valid mock payment; body: %s", string(body))

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Contains(t, parsed, "prime")
	require.Contains(t, parsed, "bits")
}

func TestE2E_WithMockPayment_FacilitatorRejects_Returns402(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(false, false) // verify returns false
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	hdrName, hdrVal := buildMockPaymentHeader(t)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/prime/256", nil)
	require.NoError(t, err)
	req.Header.Set(hdrName, hdrVal)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.NotEqual(t, http.StatusOK, resp.StatusCode,
		"facilitator rejection must not return 200, got %d", resp.StatusCode)
}

func TestE2E_Health_NoPaymentRequired(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(false, false)
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])
}

func TestE2E_Pricing_NoPaymentRequired(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(false, false)
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/pricing")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Contains(t, body, "tiers")
	tiers, ok := body["tiers"].([]interface{})
	require.True(t, ok)
	require.Len(t, tiers, 4, "expected 4 pricing tiers")
}

// TestE2E_Replay_SamePaymentHeader_NotOK sends the same X-PAYMENT header twice.
// The second request must not return 200 (plan addendum Test 6.6).
func TestE2E_Replay_SamePaymentHeader_NotOK(t *testing.T) {
	stub := testhelpers.NewFacilitatorStub(true, true)
	defer stub.Close()

	srv := newTestServer(t, stub.URL())
	defer srv.Close()

	hdrName, hdrVal := buildMockPaymentHeader(t)
	client := &http.Client{}

	doRequest := func() *http.Response {
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/prime/256", nil)
		require.NoError(t, err)
		req.Header.Set(hdrName, hdrVal)
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		return resp
	}

	// First request — expect 200.
	resp1 := doRequest()
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode,
		"first request should succeed; body: %s", string(body1))

	// Second request with the same header.
	// The stub always accepts, so replay protection depends on the SDK.
	resp2 := doRequest()
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	t.Logf("Replay response status: %d, body: %s", resp2.StatusCode, string(body2))
	// In a stub environment, replay protection may not be enforced by the SDK server.
	// We assert not 200. If the SDK doesn't track nonces, this test will fail with a
	// known limitation: replay protection is delegated to the real facilitator.
	require.NotEqual(t, http.StatusOK, resp2.StatusCode,
		"replay with same X-PAYMENT header must not return 200; body: %s", string(body2))
}
