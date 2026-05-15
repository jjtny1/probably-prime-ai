//go:build live_e2e
// +build live_e2e

// Package e2e_test contains live end-to-end tests that run against Base Sepolia.
// These tests require a funded test wallet and real Base Sepolia USDC.
//
// Run with:
//
//	export LIVE_E2E=1
//	export LIVE_E2E_PRIVATE_KEY=0x...
//	export LIVE_E2E_RPC_URL=https://sepolia.base.org
//	export X402_PAY_TO=0x<your-test-address>
//	go test -tags live_e2e ./test/e2e/... -v -run TestLive -timeout 300s
package e2e_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	x402core "github.com/x402-foundation/x402/go"
	x402http "github.com/x402-foundation/x402/go/http"
	evmexact "github.com/x402-foundation/x402/go/mechanisms/evm/exact/client"
	evmsigners "github.com/x402-foundation/x402/go/signers/evm"

	"github.com/jjtny1/probably-prime-ai/internal/config"
	"github.com/jjtny1/probably-prime-ai/internal/handler"
	"github.com/jjtny1/probably-prime-ai/internal/prime"
	"github.com/jjtny1/probably-prime-ai/internal/x402middleware"
)

// txHashRE matches a 0x-prefixed 32-byte (64 hex char) transaction hash.
var txHashRE = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

// liveEnvGate skips the test if LIVE_E2E=1 is not set.
func liveEnvGate(t *testing.T) {
	t.Helper()
	if os.Getenv("LIVE_E2E") != "1" {
		t.Skip("LIVE_E2E not set; set LIVE_E2E=1 to run live e2e tests")
	}
}

// livePayTo returns the pay-to address from the environment.
func livePayTo(t *testing.T) string {
	t.Helper()
	payTo := os.Getenv("X402_PAY_TO")
	require.NotEmpty(t, payTo, "X402_PAY_TO must be set for live e2e tests")
	return payTo
}

// livePrimeGen wraps prime.Generate for the live test server.
type livePrimeGen struct{}

func (g *livePrimeGen) Generate(bits int) (*big.Int, error) {
	return prime.Generate(bits, rand.Reader, 10000)
}

// newLiveTestServer creates a server wired to the real x402.org/facilitator.
// It registers GET /prime/{bits} (path-based routing) matching the RoutesConfig.
func newLiveTestServer(t *testing.T, payTo string) *httptest.Server {
	t.Helper()

	cfg := &config.Config{
		Port:                      4021,
		Network:                   "base-sepolia",
		PayTo:                     payTo,
		FacilitatorURL:            "https://x402.org/facilitator",
		Scheme:                    "exact",
		PrimeMaxGenerationRetries: 10000,
	}

	gen := &livePrimeGen{}
	primeH := handler.NewPrimeHandler(gen)

	facilitatorClient := x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{
		URL: cfg.FacilitatorURL,
	})

	// Register per-tier paths matching RoutesConfig keys (GET /prime/256 etc.)
	innerMux := http.NewServeMux()
	innerMux.Handle("GET /prime/{bits}", primeH)

	protectedHandler := x402middleware.Wrap(innerMux, cfg, facilitatorClient)

	outerMux := http.NewServeMux()
	outerMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	outerMux.Handle("/", protectedHandler)

	return httptest.NewServer(outerMux)
}

// newX402HTTPClient constructs a wrapped *http.Client that signs x402 payments
// using the given private key. The key is consumed by the signer; no copy is retained.
func newX402HTTPClient(t *testing.T, privateKeyHex string) *http.Client {
	t.Helper()

	// Create EVM signer from the hex-encoded private key.
	signer, err := evmsigners.NewClientSignerFromPrivateKey(privateKeyHex)
	require.NoError(t, err, "failed to create EVM signer from LIVE_E2E_PRIVATE_KEY")

	// Build x402 core client, registering the exact EVM scheme for all EVM networks.
	x402Client := x402core.Newx402Client().
		Register("eip155:*", evmexact.NewExactEvmScheme(signer, nil))

	// Wrap a standard http.Client with x402 payment handling.
	return x402http.WrapHTTPClientWithPayment(
		&http.Client{},
		x402http.Newx402HTTPClient(x402Client),
	)
}

// primeResponse is the JSON shape returned by a successful GET /prime/{bits} request.
type primeResponse struct {
	Bits         int    `json:"bits"`
	Prime        string `json:"prime"`
	PrimeDecimal string `json:"prime_decimal"`
	Rounds       int    `json:"rounds"`
	GeneratedAt  string `json:"generated_at"`
}

// assertPaymentResponse checks the PAYMENT-RESPONSE (v2) or X-PAYMENT-RESPONSE (v1)
// header on the response, decodes it, and asserts it contains a valid tx hash.
func assertPaymentResponse(t *testing.T, resp *http.Response) {
	t.Helper()

	// Accept either the v2 or v1 settlement header name.
	headerVal := resp.Header.Get("PAYMENT-RESPONSE")
	if headerVal == "" {
		headerVal = resp.Header.Get("X-PAYMENT-RESPONSE")
	}
	require.NotEmpty(t, headerVal,
		"expected PAYMENT-RESPONSE (or X-PAYMENT-RESPONSE) header on 200 response")

	// Decode base64 → JSON → SettleResponse.
	raw, err := base64.StdEncoding.DecodeString(headerVal)
	require.NoError(t, err, "PAYMENT-RESPONSE header must be valid base64")

	var settle x402core.SettleResponse
	require.NoError(t, json.Unmarshal(raw, &settle),
		"PAYMENT-RESPONSE header must decode to a valid SettlementResponse JSON")

	require.True(t, settle.Success,
		"SettlementResponse.Success must be true")

	require.NotEmpty(t, settle.Transaction,
		"SettlementResponse.Transaction must be non-empty")

	require.True(t, txHashRE.MatchString(settle.Transaction),
		"SettlementResponse.Transaction %q must be a 0x-prefixed 32-byte hex tx hash",
		settle.Transaction)
}

// assertPrimeBody reads and validates the body of a successful /prime/{bits} response.
func assertPrimeBody(t *testing.T, body io.Reader, wantBits int) {
	t.Helper()

	var pr primeResponse
	require.NoError(t, json.NewDecoder(body).Decode(&pr),
		"200 body must decode as prime JSON")

	require.Equal(t, wantBits, pr.Bits,
		"response bits field must match requested bits")

	require.Truef(t, len(pr.Prime) > 2 && pr.Prime[:2] == "0x",
		"prime field %q must start with 0x", pr.Prime)

	// Parse the hex-encoded prime value.
	hexStr := pr.Prime[2:] // strip 0x
	n := new(big.Int)
	_, ok := n.SetString(hexStr, 16)
	require.True(t, ok, "prime field must be valid hex after 0x prefix")

	// Confirm the integer has the correct bit length.
	require.Equal(t, wantBits, n.BitLen(),
		"decoded prime integer bit length must equal requested bits")

	// Cross-check primality with our own IsProbablePrime.
	isPrime, err := prime.IsProbablePrime(n, rand.Reader)
	require.NoError(t, err, "IsProbablePrime cross-check must not error")
	require.True(t, isPrime,
		"decoded prime integer must pass IsProbablePrime cross-check")
}

// TestLive_FullPaymentRoundTrip_256Bit exercises the full payment round-trip
// for a 256-bit prime request against Base Sepolia.
//
// Prerequisites:
//   - LIVE_E2E=1
//   - LIVE_E2E_PRIVATE_KEY=0x<64-hex> (funded Base Sepolia wallet)
//   - LIVE_E2E_RPC_URL=https://sepolia.base.org (or other Base Sepolia RPC)
//   - X402_PAY_TO=0x<address> (receiver wallet — can be the test wallet itself)
//
// Note: This test spends real Base Sepolia USDC (~$0.001 per run).
func TestLive_FullPaymentRoundTrip_256Bit(t *testing.T) {
	liveEnvGate(t)
	payTo := livePayTo(t)

	privateKey := os.Getenv("LIVE_E2E_PRIVATE_KEY")
	require.NotEmpty(t, privateKey, "LIVE_E2E_PRIVATE_KEY must be set")

	rpcURL := os.Getenv("LIVE_E2E_RPC_URL")
	require.NotEmpty(t, rpcURL, "LIVE_E2E_RPC_URL must be set")

	srv := newLiveTestServer(t, payTo)
	defer srv.Close()

	t.Logf("Live test server: %s", srv.URL)
	t.Logf("Facilitator: https://x402.org/facilitator")
	t.Logf("Pay-to: %s", payTo)

	// Build an x402-capable HTTP client signed with the test wallet's private key.
	// The client automatically handles the 402→sign→retry flow.
	httpClient := newX402HTTPClient(t, privateKey)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/prime/256", nil)
	require.NoError(t, err)

	resp, err := httpClient.Do(req)
	require.NoError(t, err, "x402 client must complete the payment round-trip")
	defer resp.Body.Close()

	// Assert: HTTP 200
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"x402 payment round-trip must yield HTTP 200")

	// Assert: body is a valid 256-bit prime.
	assertPrimeBody(t, resp.Body, 256)

	// Assert: PAYMENT-RESPONSE header present, base64-decodes to a SettleResponse
	// whose Transaction field is a non-empty 0x-prefixed 32-byte hex string.
	assertPaymentResponse(t, resp)
}

// TestLive_FullPaymentRoundTrip_1024Bit exercises the full payment round-trip
// for a 1024-bit prime request (higher price tier: $0.01).
func TestLive_FullPaymentRoundTrip_1024Bit(t *testing.T) {
	liveEnvGate(t)
	payTo := livePayTo(t)

	privateKey := os.Getenv("LIVE_E2E_PRIVATE_KEY")
	require.NotEmpty(t, privateKey, "LIVE_E2E_PRIVATE_KEY must be set")

	rpcURL := os.Getenv("LIVE_E2E_RPC_URL")
	require.NotEmpty(t, rpcURL, "LIVE_E2E_RPC_URL must be set")

	srv := newLiveTestServer(t, payTo)
	defer srv.Close()

	t.Logf("Live test server: %s", srv.URL)
	t.Logf("Facilitator: https://x402.org/facilitator")
	t.Logf("Pay-to: %s", payTo)

	// Build an x402-capable HTTP client signed with the test wallet's private key.
	httpClient := newX402HTTPClient(t, privateKey)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/prime/1024", nil)
	require.NoError(t, err)

	resp, err := httpClient.Do(req)
	require.NoError(t, err, "x402 client must complete the payment round-trip for 1024-bit request")
	defer resp.Body.Close()

	// Assert: HTTP 200
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"x402 payment round-trip must yield HTTP 200 for 1024-bit request")

	// Assert: body is a valid 1024-bit prime.
	assertPrimeBody(t, resp.Body, 1024)

	// Assert: PAYMENT-RESPONSE header present with valid tx hash.
	assertPaymentResponse(t, resp)
}

// TestLive_NoPayment_Returns402 verifies that the live server correctly
// returns 402 for unpaid requests against the real facilitator.
func TestLive_NoPayment_Returns402(t *testing.T) {
	liveEnvGate(t)
	payTo := livePayTo(t)

	srv := newLiveTestServer(t, payTo)
	defer srv.Close()

	for _, bits := range []int{256, 512, 1024, 2048} {
		bits := bits
		t.Run(fmt.Sprintf("bits=%d", bits), func(t *testing.T) {
			resp, err := http.Get(srv.URL + fmt.Sprintf("/prime/%d", bits))
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusPaymentRequired, resp.StatusCode,
				"live server should 402 for bits=%d", bits)
			t.Logf("bits=%d: correctly returned 402", bits)
		})
	}
}
