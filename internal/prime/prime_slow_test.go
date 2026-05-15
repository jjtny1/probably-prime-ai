//go:build slow

package prime

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGenerate_2048Bit_Smoke generates one 2048-bit prime and verifies it.
// Run with: go test -tags slow ./internal/prime/...
func TestGenerate_2048Bit_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("slow test skipped in short mode")
	}
	p, err := Generate(2048, rand.Reader, 10000)
	require.NoError(t, err)
	require.Equal(t, 2048, p.BitLen())
	isPrime, err := IsProbablePrime(p, rand.Reader)
	require.NoError(t, err)
	require.True(t, isPrime)
}
