// Package prime provides cryptographically-secure probable prime generation
// and Miller-Rabin primality testing. All randomness comes from crypto/rand
// (or an injected io.Reader for testing). math/rand is never used here.
package prime

import (
	"errors"
	"fmt"
	"io"
	"math/big"
)

// ErrRetryBudgetExceeded is returned when Generate cannot find a prime within
// the allowed number of candidate draws.
var ErrRetryBudgetExceeded = errors.New("prime generation failed: retry budget exceeded")

// Generate returns a random probable prime of exactly `bits` bits.
// The top two bits are forced set (so two such primes always produce a 2*bits product).
// The bottom bit is forced set (odd).
//
// `bits` must be >= 8. The handler restricts external callers to {256,512,1024,2048}.
// `rnd` is the source of randomness; use crypto/rand.Reader in production.
// `maxRetries` caps the number of candidate draws; use Config.PrimeMaxGenerationRetries.
func Generate(bits int, rnd io.Reader, maxRetries int) (*big.Int, error) {
	if bits < 8 {
		return nil, fmt.Errorf("prime.Generate: bits must be >= 8, got %d", bits)
	}

	byteLen := (bits + 7) / 8
	buf := make([]byte, byteLen)

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := io.ReadFull(rnd, buf)
		if err != nil {
			return nil, fmt.Errorf("prime.Generate: failed to read random bytes: %w", err)
		}

		// Force the top two bits set.
		// This ensures the generated number has exactly `bits` bits and that
		// the product of two such primes has exactly 2*bits bits.
		buf[0] |= 0xC0

		// If bits is not a multiple of 8, the top byte only uses (bits%8) bits.
		// Shift the top byte right so only the low (bits%8) bits are active,
		// then set the top two bits within that range.
		if rem := bits % 8; rem != 0 {
			// Clear high bits that are outside our bit range.
			mask := byte(0xFF >> (8 - rem))
			buf[0] &= mask
			// Set top two bits within the range.
			if rem >= 2 {
				buf[0] |= byte(0xC0 >> (8 - rem))
			} else {
				// rem == 1: only one bit available, just set it.
				buf[0] |= byte(1)
			}
		}

		// Force the bottom bit set (odd).
		buf[byteLen-1] |= 0x01

		candidate := new(big.Int).SetBytes(buf)

		// Verify the candidate has exactly `bits` bits.
		if candidate.BitLen() != bits {
			continue
		}

		isPrime, err := IsProbablePrime(candidate, rnd)
		if err != nil {
			return nil, fmt.Errorf("prime.Generate: primality check failed: %w", err)
		}
		if isPrime {
			return candidate, nil
		}
	}

	return nil, fmt.Errorf("%w after %d attempts", ErrRetryBudgetExceeded, maxRetries)
}
