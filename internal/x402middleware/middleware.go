// Package x402middleware adapts the x402 SDK payment middleware to this service's configuration.
// It builds RoutesConfig from pricing tiers and wraps an http.Handler with payment enforcement.
package x402middleware

import (
	"fmt"
	"net/http"

	x402 "github.com/x402-foundation/x402/go"
	x402http "github.com/x402-foundation/x402/go/http"
	"github.com/x402-foundation/x402/go/http/nethttp"
	evmserver "github.com/x402-foundation/x402/go/mechanisms/evm/exact/server"

	"github.com/jjtny1/probably-prime-ai/internal/config"
	"github.com/jjtny1/probably-prime-ai/internal/pricing"
)

// networkToCAIP2 maps our human-readable network names to CAIP-2 EIP-155 identifiers.
var networkToCAIP2 = map[string]x402.Network{
	"base-sepolia": "eip155:84532",
	"base":         "eip155:8453",
}

// BuildRoutesConfig constructs the x402 RoutesConfig for this service.
// It creates one route entry per allowed bit size, keyed "GET /prime/{bits}".
// Each entry carries the tier-specific price so the on-chain charge is correct
// regardless of which path the client requests.
func BuildRoutesConfig(cfg *config.Config) x402http.RoutesConfig {
	network := networkToCAIP2[cfg.Network]
	routes := make(x402http.RoutesConfig, len(pricing.AllowedBits()))

	for _, bits := range pricing.AllowedBits() {
		price, _ := pricing.Tier(bits)
		key := fmt.Sprintf("GET /prime/%d", bits)
		routes[key] = x402http.RouteConfig{
			Accepts: x402http.PaymentOptions{
				{
					Scheme:  cfg.Scheme,
					PayTo:   cfg.PayTo,
					Price:   price,
					Network: network,
				},
			},
			Description: fmt.Sprintf("Generate a %d-bit probable prime", bits),
			MimeType:    "application/json",
		}
	}

	return routes
}

// Wrap applies the x402 payment middleware to the given handler using the provided
// configuration and facilitator client. Requests that don't require payment pass
// through unchanged.
func Wrap(handler http.Handler, cfg *config.Config, facilitatorClient x402.FacilitatorClient) http.Handler {
	routes := BuildRoutesConfig(cfg)
	network := networkToCAIP2[cfg.Network]

	middleware := nethttp.PaymentMiddlewareFromConfig(
		routes,
		nethttp.WithFacilitatorClient(facilitatorClient),
		nethttp.WithScheme(network, evmserver.NewExactEvmScheme()),
		nethttp.WithSyncFacilitatorOnStart(true),
	)

	return middleware(handler)
}
