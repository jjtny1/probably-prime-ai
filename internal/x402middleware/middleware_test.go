package x402middleware_test

import (
	"fmt"
	"testing"

	"github.com/jjtny1/probably-prime-ai/internal/config"
	"github.com/jjtny1/probably-prime-ai/internal/pricing"
	"github.com/jjtny1/probably-prime-ai/internal/x402middleware"
	"github.com/stretchr/testify/require"
)

func makeConfig(payTo, network string) *config.Config {
	return &config.Config{
		Port:                      4021,
		Network:                   network,
		PayTo:                     payTo,
		FacilitatorURL:            "https://x402.org/facilitator",
		Scheme:                    "exact",
		PrimeMaxGenerationRetries: 10000,
	}
}

// TestBuildRoutesConfig_AllTiersPresent verifies that the returned RoutesConfig
// has exactly one entry per allowed bit size, keyed "GET /prime/256",
// "GET /prime/512", "GET /prime/1024", "GET /prime/2048".
// This is plan Test 6.1.
func TestBuildRoutesConfig_AllTiersPresent(t *testing.T) {
	cfg := makeConfig("0x000000000000000000000000000000000000000A", "base-sepolia")
	routes := x402middleware.BuildRoutesConfig(cfg)

	allowedBits := pricing.AllowedBits() // {256, 512, 1024, 2048}
	require.Len(t, routes, len(allowedBits), "expected exactly one route per allowed bit size")

	for _, bits := range allowedBits {
		key := fmt.Sprintf("GET /prime/%d", bits)
		route, ok := routes[key]
		require.True(t, ok, "expected key %q in RoutesConfig", key)
		require.NotEmpty(t, route.Accepts, "route %q must have at least one payment option", key)
	}
}

// TestBuildRoutesConfig_PricesMatchTiers verifies that each route entry's
// Accepts[0].Price equals the tier-specific price string for that bit size.
// This is plan Test 6.2.
func TestBuildRoutesConfig_PricesMatchTiers(t *testing.T) {
	cfg := makeConfig("0x000000000000000000000000000000000000000A", "base-sepolia")
	routes := x402middleware.BuildRoutesConfig(cfg)

	wantPrices := map[int]string{
		256:  "$0.001",
		512:  "$0.003",
		1024: "$0.01",
		2048: "$0.05",
	}

	for _, bits := range pricing.AllowedBits() {
		bits := bits
		key := fmt.Sprintf("GET /prime/%d", bits)
		route, ok := routes[key]
		require.True(t, ok, "key %q must be present", key)
		require.NotEmpty(t, route.Accepts, "route %q must have payment options", key)

		price := route.Accepts[0].Price
		require.NotNil(t, price, "price for %q must not be nil", key)
		priceStr, ok := price.(string)
		require.True(t, ok, "price for %q must be a string, got %T", key, price)
		require.Equal(t, wantPrices[bits], priceStr,
			"price for %q must match tier definition", key)
	}
}

// TestBuildRoutesConfig_NetworkMapping verifies that the network string is converted
// to the correct CAIP-2 EIP-155 chain ID across all tier routes.
// This is plan Test 6.3.
func TestBuildRoutesConfig_NetworkMapping(t *testing.T) {
	cases := []struct {
		network   string
		wantCaip2 string
	}{
		{"base-sepolia", "eip155:84532"},
		{"base", "eip155:8453"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.network, func(t *testing.T) {
			cfg := makeConfig("0x000000000000000000000000000000000000000A", tc.network)
			routes := x402middleware.BuildRoutesConfig(cfg)

			for _, bits := range pricing.AllowedBits() {
				bits := bits
				key := fmt.Sprintf("GET /prime/%d", bits)
				route, ok := routes[key]
				require.True(t, ok, "key %q must be present for network %s", key, tc.network)
				require.NotEmpty(t, route.Accepts)
				require.Equal(t, tc.wantCaip2, string(route.Accepts[0].Network),
					"network mismatch for %s route %s", tc.network, key)
			}
		})
	}
}

// TestBuildRoutesConfig_PayToPropagated verifies that the PayTo address from config
// is present in every tier route's payment option.
// This is plan Test 6.4.
func TestBuildRoutesConfig_PayToPropagated(t *testing.T) {
	payTo := "0x000000000000000000000000000000000000000A"
	cfg := makeConfig(payTo, "base-sepolia")
	routes := x402middleware.BuildRoutesConfig(cfg)

	for _, bits := range pricing.AllowedBits() {
		bits := bits
		key := fmt.Sprintf("GET /prime/%d", bits)
		route, ok := routes[key]
		require.True(t, ok, "key %q must be present", key)
		require.NotEmpty(t, route.Accepts)
		require.Equal(t, payTo, route.Accepts[0].PayTo,
			"PayTo mismatch for route %q", key)
	}
}

// TestBuildRoutesConfig_SchemeIsExact verifies that every tier route's
// payment scheme is "exact".
// This is plan Test 6.5.
func TestBuildRoutesConfig_SchemeIsExact(t *testing.T) {
	cfg := makeConfig("0x000000000000000000000000000000000000000A", "base-sepolia")
	routes := x402middleware.BuildRoutesConfig(cfg)

	for _, bits := range pricing.AllowedBits() {
		bits := bits
		key := fmt.Sprintf("GET /prime/%d", bits)
		route, ok := routes[key]
		require.True(t, ok, "key %q must be present", key)
		require.NotEmpty(t, route.Accepts)
		require.Equal(t, "exact", route.Accepts[0].Scheme,
			"scheme must be 'exact' for route %q", key)
	}
}
