// Package config loads and validates server configuration from environment variables.
package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort           = 4021
	defaultNetwork        = "base-sepolia"
	defaultScheme         = "exact"
	defaultFacilitatorURL = "https://x402.org/facilitator"
	defaultMaxRetries     = 10000
)

// validNetworks is the set of allowed X402_NETWORK values.
var validNetworks = map[string]bool{
	"base-sepolia": true,
	"base":         true,
}

// Config holds the validated server configuration.
type Config struct {
	Port                      uint16
	Network                   string
	PayTo                     string
	FacilitatorURL            string
	Scheme                    string
	PrimeMaxGenerationRetries int
}

// Load reads environment variables, applies defaults, validates all fields,
// and returns a fully populated Config or an error describing the first failure.
// It never panics.
func Load() (*Config, error) {
	cfg := &Config{}

	// PORT
	portStr := os.Getenv("PORT")
	if portStr == "" {
		cfg.Port = defaultPort
	} else {
		p, err := strconv.ParseUint(portStr, 10, 64)
		if err != nil || p == 0 || p > 65535 {
			return nil, fmt.Errorf("invalid PORT %q: must be an integer in [1, 65535]", portStr)
		}
		cfg.Port = uint16(p)
	}

	// X402_NETWORK
	network := os.Getenv("X402_NETWORK")
	if network == "" {
		cfg.Network = defaultNetwork
	} else if !validNetworks[network] {
		return nil, fmt.Errorf("invalid X402_NETWORK %q: must be one of base-sepolia, base", network)
	} else {
		cfg.Network = network
	}

	// X402_PAY_TO (required)
	payTo := os.Getenv("X402_PAY_TO")
	if err := validateEthAddress(payTo); err != nil {
		return nil, fmt.Errorf("invalid X402_PAY_TO: %w", err)
	}
	cfg.PayTo = payTo

	// X402_FACILITATOR_URL
	facilitatorURL := os.Getenv("X402_FACILITATOR_URL")
	if facilitatorURL == "" {
		cfg.FacilitatorURL = defaultFacilitatorURL
	} else {
		u, err := url.Parse(facilitatorURL)
		if err != nil || u.Scheme != "https" {
			return nil, fmt.Errorf("invalid X402_FACILITATOR_URL %q: must be an https:// URL", facilitatorURL)
		}
		cfg.FacilitatorURL = facilitatorURL
	}

	// X402_SCHEME
	scheme := os.Getenv("X402_SCHEME")
	if scheme == "" {
		cfg.Scheme = defaultScheme
	} else {
		cfg.Scheme = scheme
	}

	// PRIME_MAX_GENERATION_RETRIES
	retriesStr := os.Getenv("PRIME_MAX_GENERATION_RETRIES")
	if retriesStr == "" {
		cfg.PrimeMaxGenerationRetries = defaultMaxRetries
	} else {
		r, err := strconv.Atoi(retriesStr)
		if err != nil || r <= 0 {
			return nil, fmt.Errorf("invalid PRIME_MAX_GENERATION_RETRIES %q: must be a positive integer", retriesStr)
		}
		cfg.PrimeMaxGenerationRetries = r
	}

	return cfg, nil
}

// validateEthAddress checks that addr is a 0x-prefixed 40-hex-char Ethereum address.
func validateEthAddress(addr string) error {
	if !strings.HasPrefix(addr, "0x") {
		return fmt.Errorf("X402_PAY_TO must start with 0x (got %q)", addr)
	}
	hexPart := addr[2:]
	if len(hexPart) != 40 {
		return fmt.Errorf("X402_PAY_TO must have exactly 40 hex chars after 0x (got %d)", len(hexPart))
	}
	if _, err := hex.DecodeString(hexPart); err != nil {
		return fmt.Errorf("X402_PAY_TO contains non-hex characters: %w", err)
	}
	return nil
}
