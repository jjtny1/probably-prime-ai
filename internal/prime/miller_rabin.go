package prime

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
)

// smallPrimeProducts is a product of small primes used for quick trial division.
// We trial-divide by primes up to 1000 using GCD to avoid large divisions.
var smallPrimesForSieve []uint64

func init() {
	smallPrimesForSieve = sievePrimesUpTo(1000)
}

// sievePrimesUpTo returns all primes up to n using the sieve of Eratosthenes.
func sievePrimesUpTo(n int) []uint64 {
	sieve := make([]bool, n+1)
	for i := 2; i <= n; i++ {
		if sieve[i] {
			continue
		}
		for j := i * i; j <= n; j += i {
			sieve[j] = true
		}
	}
	var primes []uint64
	for i := 2; i <= n; i++ {
		if !sieve[i] {
			primes = append(primes, uint64(i))
		}
	}
	return primes
}

// IsProbablePrime returns (true, nil) if n is probably prime, (false, nil) if
// n is definitely composite, and (false, err) if randomness could not be read.
//
// The implementation uses:
//  1. Trivial checks (≤1, even, trial division by small primes).
//  2. Deterministic witnesses for n < 3,317,044,064,679,887,385,961,981 (Sorenson & Webster 2017).
//  3. randomized Miller-Rabin with roundsFor(n.BitLen()) witnesses otherwise.
//
// The injected io.Reader is used only for witness selection when random witnesses
// are required (n ≥ boundary). Pass crypto/rand.Reader in production; pass a
// deterministic reader in tests for reproducibility.
func IsProbablePrime(n *big.Int, rnd io.Reader) (bool, error) {
	// Step 1: Trivial cases.
	if n.Sign() <= 0 || n.Cmp(bigOne) == 0 {
		return false, nil
	}
	if n.Cmp(bigTwo) == 0 || n.Cmp(bigThree) == 0 {
		return true, nil
	}
	if n.Bit(0) == 0 {
		// even and > 2
		return false, nil
	}

	// Step 2: Trial division by small primes up to 1000.
	nMod := new(big.Int)
	for _, p := range smallPrimesForSieve {
		bigP := new(big.Int).SetUint64(p)
		if n.Cmp(bigP) == 0 {
			return true, nil // n is one of the small primes
		}
		nMod.Mod(n, bigP)
		if nMod.Sign() == 0 {
			return false, nil // divisible by a small prime
		}
	}

	// Step 3: Decompose n-1 = 2^r * d with d odd.
	nMinus1 := new(big.Int).Sub(n, bigOne)
	r, d := decomposeEven(nMinus1)

	// Step 4: Choose witness set.
	witnesses := deterministicWitnesses(n)
	if witnesses != nil {
		// Deterministic path.
		for _, a := range witnesses {
			bigA := new(big.Int).SetUint64(a)
			if !millerRabinWitness(bigA, n, d, r) {
				return false, nil
			}
		}
		return true, nil
	}

	// Randomized path.
	rounds := roundsFor(n.BitLen())
	nMinus2 := new(big.Int).Sub(n, bigTwo)
	for i := 0; i < rounds; i++ {
		// Draw a random witness a in [2, n-2].
		a, err := randBigInt(rnd, nMinus2)
		if err != nil {
			return false, fmt.Errorf("IsProbablePrime: failed to read random witness: %w", err)
		}
		// Ensure a >= 2
		if a.Cmp(bigTwo) < 0 {
			a.Set(bigTwo)
		}
		if !millerRabinWitness(a, n, d, r) {
			return false, nil
		}
	}
	return true, nil
}

// millerRabinWitness performs one Miller-Rabin round for witness a and candidate n.
// n-1 = 2^r * d with d odd. Returns true if a is not a witness of compositeness.
func millerRabinWitness(a, n, d *big.Int, r int) bool {
	nMinus1 := new(big.Int).Sub(n, bigOne)
	x := new(big.Int).Exp(a, d, n) // x = a^d mod n

	if x.Cmp(bigOne) == 0 || x.Cmp(nMinus1) == 0 {
		return true
	}
	for i := 1; i < r; i++ {
		x.Exp(x, bigTwo, n)
		if x.Cmp(nMinus1) == 0 {
			return true
		}
	}
	return false
}

// decomposeEven factors out powers of 2 from n: returns (r, d) such that n = 2^r * d and d is odd.
func decomposeEven(n *big.Int) (int, *big.Int) {
	d := new(big.Int).Set(n)
	r := 0
	for d.Bit(0) == 0 {
		d.Rsh(d, 1)
		r++
	}
	return r, d
}

// randBigInt returns a random *big.Int in [0, max) using rnd.
// Falls back to crypto/rand if rnd runs out of data or returns an error.
func randBigInt(rnd io.Reader, max *big.Int) (*big.Int, error) {
	if rnd == rand.Reader {
		return cryptoRandInt(max)
	}
	// Try the injected reader; if it errors fall back to crypto/rand.
	n, err := cryptoRandIntFrom(rnd, max)
	if err != nil {
		return cryptoRandInt(max)
	}
	return n, nil
}

// cryptoRandInt generates a random big.Int in [0, max) using crypto/rand.
func cryptoRandInt(max *big.Int) (*big.Int, error) {
	return cryptoRandIntFrom(rand.Reader, max)
}

// cryptoRandIntFrom generates a random big.Int in [0, max) using the given reader.
func cryptoRandIntFrom(rnd io.Reader, max *big.Int) (*big.Int, error) {
	// Use rejection sampling: generate a random number with max.BitLen() bits.
	bits := max.BitLen()
	if bits == 0 {
		return big.NewInt(0), nil
	}
	bytes := (bits + 7) / 8
	buf := make([]byte, bytes)
	_, err := io.ReadFull(rnd, buf)
	if err != nil {
		return nil, err
	}
	// Clear high bits to ensure result < 2^bits.
	mask := byte(0xFF >> (bytes*8 - bits))
	buf[0] &= mask

	n := new(big.Int).SetBytes(buf)
	if n.Cmp(max) >= 0 {
		// Rejection: reduce modulo max (slightly biased but fine for witnesses).
		n.Mod(n, max)
	}
	return n, nil
}

// Package-level big.Int constants to avoid repeated allocation.
var (
	bigOne   = big.NewInt(1)
	bigTwo   = big.NewInt(2)
	bigThree = big.NewInt(3)
)
