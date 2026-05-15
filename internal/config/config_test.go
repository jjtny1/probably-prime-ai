package config_test

import (
	"testing"

	"github.com/jjtny1/probably-prime-ai/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoad_HappyPath_Defaults(t *testing.T) {
	t.Setenv("X402_PAY_TO", "0x0000000000000000000000000000000000000001")
	// Clear optional vars so defaults apply
	t.Setenv("PORT", "")
	t.Setenv("X402_NETWORK", "")
	t.Setenv("X402_SCHEME", "")
	t.Setenv("X402_FACILITATOR_URL", "")
	t.Setenv("PRIME_MAX_GENERATION_RETRIES", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, uint16(4021), cfg.Port)
	require.Equal(t, "base-sepolia", cfg.Network)
	require.Equal(t, "exact", cfg.Scheme)
	require.Equal(t, "0x0000000000000000000000000000000000000001", cfg.PayTo)
	require.Equal(t, 10000, cfg.PrimeMaxGenerationRetries)
}

func TestLoad_MissingPayTo(t *testing.T) {
	t.Setenv("X402_PAY_TO", "")
	cfg, err := config.Load()
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "X402_PAY_TO")
}

func TestLoad_InvalidPayTo_Table(t *testing.T) {
	cases := []string{
		"",
		"not-hex",
		"0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
		"0x123",
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	}
	for _, addr := range cases {
		addr := addr
		t.Run(addr, func(t *testing.T) {
			t.Setenv("X402_PAY_TO", addr)
			cfg, err := config.Load()
			require.Nil(t, cfg, "expected nil config for bad address %q", addr)
			require.Error(t, err, "expected error for bad address %q", addr)
		})
	}
}

func TestLoad_InvalidNetwork(t *testing.T) {
	t.Setenv("X402_PAY_TO", "0x0000000000000000000000000000000000000001")
	t.Setenv("X402_NETWORK", "ethereum")
	cfg, err := config.Load()
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "X402_NETWORK")
}

func TestLoad_ValidNetworks_Table(t *testing.T) {
	t.Setenv("X402_PAY_TO", "0x0000000000000000000000000000000000000001")
	for _, network := range []string{"base-sepolia", "base"} {
		network := network
		t.Run(network, func(t *testing.T) {
			t.Setenv("X402_NETWORK", network)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.Equal(t, network, cfg.Network)
		})
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("X402_PAY_TO", "0x0000000000000000000000000000000000000001")
	for _, port := range []string{"abc", "0", "70000"} {
		port := port
		t.Run(port, func(t *testing.T) {
			t.Setenv("PORT", port)
			cfg, err := config.Load()
			require.Nil(t, cfg)
			require.Error(t, err)
		})
	}
}

func TestLoad_InvalidFacilitatorURL(t *testing.T) {
	t.Setenv("X402_PAY_TO", "0x0000000000000000000000000000000000000001")
	t.Setenv("X402_FACILITATOR_URL", "ftp://x")
	cfg, err := config.Load()
	require.Nil(t, cfg)
	require.Error(t, err)
}

func TestLoad_MaxRetriesNonPositive(t *testing.T) {
	t.Setenv("X402_PAY_TO", "0x0000000000000000000000000000000000000001")
	for _, val := range []string{"0", "-5"} {
		val := val
		t.Run(val, func(t *testing.T) {
			t.Setenv("PRIME_MAX_GENERATION_RETRIES", val)
			cfg, err := config.Load()
			require.Nil(t, cfg)
			require.Error(t, err)
		})
	}
}
