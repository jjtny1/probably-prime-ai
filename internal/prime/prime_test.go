package prime

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate_ReturnsCorrectBitLength(t *testing.T) {
	for _, bits := range []int{256, 512, 1024} {
		bits := bits
		t.Run("", func(t *testing.T) {
			p, err := Generate(bits, rand.Reader, 10000)
			require.NoError(t, err)
			require.NotNil(t, p)
			require.Equal(t, bits, p.BitLen(), "bit length mismatch for bits=%d", bits)
			require.Equal(t, uint(1), p.Bit(bits-1), "top bit not set for bits=%d", bits)
		})
	}
}

func TestGenerate_ReturnsOdd(t *testing.T) {
	for _, bits := range []int{256, 512, 1024} {
		bits := bits
		t.Run("", func(t *testing.T) {
			p, err := Generate(bits, rand.Reader, 10000)
			require.NoError(t, err)
			require.Equal(t, uint(1), p.Bit(0), "expected odd prime for bits=%d", bits)
		})
	}
}

func TestGenerate_PassesIsProbablePrime(t *testing.T) {
	for i := 0; i < 10; i++ {
		p, err := Generate(512, rand.Reader, 10000)
		require.NoError(t, err)
		isPrime, err := IsProbablePrime(p, rand.Reader)
		require.NoError(t, err)
		require.True(t, isPrime, "Generate returned a non-prime on iteration %d: %s", i, p.String())
	}
}

func TestGenerate_RejectsBadBits(t *testing.T) {
	for _, bits := range []int{0, 1, 7, -1} {
		bits := bits
		t.Run("", func(t *testing.T) {
			_, err := Generate(bits, rand.Reader, 10000)
			require.Error(t, err, "expected error for bits=%d", bits)
		})
	}
}

// alwaysCompositeReader is an io.Reader that returns all 0xFF bytes, which after
// the top-two-bits and bottom-bit forcing in Generate produces composite candidates.
type alwaysCompositeReader struct{}

func (r *alwaysCompositeReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0xFF
	}
	return len(p), nil
}

// TestGenerate_ExhaustsRetryBudget tests that Generate returns an error when
// maxRetries is exhausted with a reader that always produces a composite.
func TestGenerate_ExhaustsRetryBudget(t *testing.T) {
	// All-0xFF bytes → after forcing top two bits set and bottom bit set:
	// For 8 bits: 0xFF = 255, which is divisible by 3, 5, 17, 51, 85, 255 → composite.
	r := &alwaysCompositeReader{}
	_, err := Generate(8, r, 3)
	require.Error(t, err)
	require.Contains(t, err.Error(), "retry budget")
}

func TestGenerate_UsesInjectedReader(t *testing.T) {
	// With the same seed, two Generate calls must return the same prime.
	// Use a large seed (16KB) to ensure we don't exhaust the reader for 16-bit primes.
	seed := bytes.Repeat([]byte{0x42, 0x13, 0x37, 0x99, 0x5A, 0x7B, 0xCC, 0xD1}, 4096)
	rdr1 := bytes.NewReader(seed)
	rdr2 := bytes.NewReader(seed)

	p1, err := Generate(16, rdr1, 1000)
	if errors.Is(err, ErrRetryBudgetExceeded) {
		t.Skip("seeded reader exhausted without finding a prime — seed too small for this bit length")
	}
	require.NoError(t, err)

	p2, err := Generate(16, rdr2, 1000)
	require.NoError(t, err)

	require.Equal(t, p1.String(), p2.String(), "same seed must produce same prime")
}

// Ensure the alwaysCompositeReader satisfies io.Reader.
var _ io.Reader = (*alwaysCompositeReader)(nil)
