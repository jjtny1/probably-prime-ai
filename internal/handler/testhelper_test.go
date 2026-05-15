package handler_test

import (
	"crypto/rand"
	"math/big"

	"github.com/jjtny1/probably-prime-ai/internal/prime"
)

// generatePrime wraps prime.Generate for use in tests.
func generatePrime(bits int) (*big.Int, error) {
	return prime.Generate(bits, rand.Reader, 10000)
}
