package pricing_test

import (
	"testing"

	"github.com/jjtny1/probably-prime-ai/internal/pricing"
	"github.com/stretchr/testify/require"
)

func TestTier_KnownBits(t *testing.T) {
	cases := []struct {
		bits      int
		wantPrice string
		wantOK    bool
	}{
		{256, "$0.001", true},
		{512, "$0.003", true},
		{1024, "$0.01", true},
		{2048, "$0.05", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			got, ok := pricing.Tier(tc.bits)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.wantPrice, got)
		})
	}
}

func TestTier_UnknownBits(t *testing.T) {
	unknowns := []int{0, -1, 1, 255, 257, 384, 768, 1023, 1025, 4096}
	for _, bits := range unknowns {
		bits := bits
		t.Run("", func(t *testing.T) {
			got, ok := pricing.Tier(bits)
			require.False(t, ok, "expected ok=false for bits=%d", bits)
			require.Equal(t, "", got)
		})
	}
}

func TestAllowedBits_Sorted(t *testing.T) {
	require.Equal(t, []int{256, 512, 1024, 2048}, pricing.AllowedBits())
}

func TestAllowedBits_NotMutable(t *testing.T) {
	first := pricing.AllowedBits()
	first[0] = 9999
	second := pricing.AllowedBits()
	require.Equal(t, []int{256, 512, 1024, 2048}, second, "AllowedBits must not be mutated by caller")
}
