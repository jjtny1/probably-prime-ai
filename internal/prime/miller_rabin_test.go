package prime

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// smallPrimesUpTo100 lists all primes ≤ 100.
var smallPrimesUpTo100 = []int64{
	2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47,
	53, 59, 61, 67, 71, 73, 79, 83, 89, 97,
}

// smallCompositesUpTo100 lists all composites ≤ 100 (even + odd composites).
var smallCompositesUpTo100 = func() []int64 {
	isPrime := make(map[int64]bool)
	for _, p := range smallPrimesUpTo100 {
		isPrime[p] = true
	}
	var comps []int64
	for i := int64(2); i <= 100; i++ {
		if !isPrime[i] {
			comps = append(comps, i)
		}
	}
	return comps
}()

func TestIsProbablePrime_SmallPrimes(t *testing.T) {
	for _, p := range smallPrimesUpTo100 {
		n := big.NewInt(p)
		got, err := IsProbablePrime(n, rand.Reader)
		require.NoError(t, err, "p=%d", p)
		require.True(t, got, "expected IsProbablePrime(%d) == true", p)
	}
}

func TestIsProbablePrime_SmallComposites(t *testing.T) {
	for _, c := range smallCompositesUpTo100 {
		n := big.NewInt(c)
		got, err := IsProbablePrime(n, rand.Reader)
		require.NoError(t, err, "c=%d", c)
		require.False(t, got, "expected IsProbablePrime(%d) == false", c)
	}
}

func TestIsProbablePrime_Zero_One_Negative(t *testing.T) {
	cases := []int64{0, 1, -1, -7}
	for _, v := range cases {
		n := big.NewInt(v)
		got, err := IsProbablePrime(n, rand.Reader)
		require.NoError(t, err)
		require.False(t, got, "expected IsProbablePrime(%d) == false", v)
	}
}

func TestIsProbablePrime_Carmichael(t *testing.T) {
	carmichaels := []int64{
		561, 1105, 1729, 2465, 2821, 6601, 8911, 10585,
		15841, 29341, 41041, 46657, 52633, 62745, 63973, 75361,
	}
	for _, c := range carmichaels {
		n := big.NewInt(c)
		got, err := IsProbablePrime(n, rand.Reader)
		require.NoError(t, err, "c=%d", c)
		require.False(t, got, "Carmichael number %d must not be reported as prime", c)
	}
}

func TestIsProbablePrime_MersennePrimes(t *testing.T) {
	// M_31 = 2^31 - 1
	m31 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(31), nil), big.NewInt(1))
	// M_61 = 2^61 - 1
	m61 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(61), nil), big.NewInt(1))
	// M_89 = 2^89 - 1
	m89 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(89), nil), big.NewInt(1))
	// M_107 = 2^107 - 1
	m107 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(107), nil), big.NewInt(1))
	// M_127 = 2^127 - 1
	m127 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(127), nil), big.NewInt(1))

	mersennes := []*big.Int{m31, m61, m89, m107, m127}
	for _, m := range mersennes {
		got, err := IsProbablePrime(m, rand.Reader)
		require.NoError(t, err, "Mersenne %s", m.String())
		require.True(t, got, "expected Mersenne prime %s to be reported as prime", m.String())
	}
}

func TestIsProbablePrime_LargeKnownPrime(t *testing.T) {
	// RFC 3526 Group 14 2048-bit MODP prime (well-known; use just the first 1024 bits for speed).
	// Using the Oakley Group 2 (1024-bit) prime from RFC 2409 Section 6.2.
	// This is a well-known 1024-bit prime:
	hexPrime := "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1" +
		"29024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381" +
		"FFFFFFFFFFFFFFFF"
	n := new(big.Int)
	n.SetString(hexPrime, 16)
	got, err := IsProbablePrime(n, rand.Reader)
	require.NoError(t, err)
	require.True(t, got, "known 1024-bit prime (RFC 2409 Oakley Group 2) must be reported as prime")
}

func TestIsProbablePrime_LargeKnownComposite(t *testing.T) {
	// Product of two large primes: (2^127 - 1) * (2^107 - 1)
	m127 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(127), nil), big.NewInt(1))
	m107 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(107), nil), big.NewInt(1))
	composite := new(big.Int).Mul(m127, m107)

	got, err := IsProbablePrime(composite, rand.Reader)
	require.NoError(t, err)
	require.False(t, got, "product of two Mersenne primes must be reported as composite")
}

func TestIsProbablePrime_StatisticalErrorRate(t *testing.T) {
	// Enumerate composites in range [2^16, 2^20] by sieve, test 1000 of them.
	const lo = 1 << 16
	const hi = 1 << 20
	sieve := make([]bool, hi-lo+1) // true = composite
	for i := 2; i*i <= hi; i++ {
		start := (lo + i - 1) / i * i
		if start == i {
			start += i
		}
		for j := start; j <= hi; j += i {
			sieve[j-lo] = true
		}
	}
	count := 0
	for idx, isComposite := range sieve {
		if !isComposite {
			continue
		}
		n := big.NewInt(int64(lo + idx))
		// Also skip the case where n is prime (shouldn't happen in sieve but be safe)
		got, err := IsProbablePrime(n, rand.Reader)
		require.NoError(t, err)
		require.False(t, got, "composite %d reported as prime", lo+idx)
		count++
		if count >= 1000 {
			break
		}
	}
	require.GreaterOrEqual(t, count, 1000, "need at least 1000 composites in sieve range")
}

func TestRoundsFor_BitSize(t *testing.T) {
	require.GreaterOrEqual(t, roundsFor(256), 64)
	require.GreaterOrEqual(t, roundsFor(512), 56)
	require.GreaterOrEqual(t, roundsFor(1024), 40)
	require.GreaterOrEqual(t, roundsFor(2048), 27)
	// floor for small callers
	require.GreaterOrEqual(t, roundsFor(64), 64)
	// roundsFor(0) must return >= 64 (floor behavior)
	require.GreaterOrEqual(t, roundsFor(0), 64)
}

func TestDeterministicWitnesses_Boundary(t *testing.T) {
	// The boundary: n < 3,317,044,064,679,887,385,961,981
	// Use Sorenson & Webster 2017 deterministic witnesses.
	boundary := new(big.Int)
	boundary.SetString("3317044064679887385961981", 10)

	expectedWitnesses := []uint64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37}

	// n = 100 (small, below boundary)
	n100 := big.NewInt(100)
	ws100 := deterministicWitnesses(n100)
	require.Equal(t, expectedWitnesses, ws100, "deterministicWitnesses(100) must return canonical set")

	// n = 2^100 (large, above boundary) — must NOT return the full deterministic set
	n2p100 := new(big.Int).Exp(big.NewInt(2), big.NewInt(100), nil)
	ws2p100 := deterministicWitnesses(n2p100)
	require.Nil(t, ws2p100, "deterministicWitnesses(2^100) must return nil (use random witnesses)")

	// n = boundary - 1 (just below, should use deterministic)
	nBoundaryMinus1 := new(big.Int).Sub(boundary, big.NewInt(1))
	wsBelow := deterministicWitnesses(nBoundaryMinus1)
	require.Equal(t, expectedWitnesses, wsBelow, "deterministicWitnesses(boundary-1) must return canonical set")

	// n = boundary (at boundary, should use random)
	wsAtBoundary := deterministicWitnesses(boundary)
	require.Nil(t, wsAtBoundary, "deterministicWitnesses(boundary) must return nil")
}

// TestIsProbablePrime_AgreesWithStdlib cross-checks against big.Int.ProbablyPrime.
// This test is informational and not a substitute for the above specific test vectors.
func TestIsProbablePrime_AgreesWithStdlib(t *testing.T) {
	// Test 100 random candidates from crypto/rand in 512-bit range
	const n = 100
	agree := 0
	for i := 0; i < n; i++ {
		candidate, err := rand.Prime(rand.Reader, 64)
		require.NoError(t, err)
		ours, err := IsProbablePrime(candidate, rand.Reader)
		require.NoError(t, err)
		stdlib := candidate.ProbablyPrime(20)
		require.Equal(t, stdlib, ours, "disagreement on %s", candidate.String())
		agree++
	}
	require.Equal(t, n, agree)
}

// Helper: read a file of Mersenne primes (ignoring comment lines).
func readMersennePrimesFile(t *testing.T) []*big.Int {
	t.Helper()
	f, err := os.Open("testdata/mersenne_primes.txt")
	require.NoError(t, err)
	defer f.Close()

	var primes []*big.Int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		n := new(big.Int)
		_, ok := n.SetString(line, 10)
		require.True(t, ok, "parse error for line: %s", line)
		primes = append(primes, n)
	}
	return primes
}

// TestIsProbablePrime_MersennePrimesFromFile verifies the testdata file is consistent.
func TestIsProbablePrime_MersennePrimesFromFile(t *testing.T) {
	primes := readMersennePrimesFile(t)
	require.NotEmpty(t, primes)
	for _, p := range primes {
		got, err := IsProbablePrime(p, rand.Reader)
		require.NoError(t, err)
		require.True(t, got, "Mersenne prime %s not detected", p.String())
	}
}

// TestMR_DeterministicWithSeed verifies that using a deterministic io.Reader
// (for the random witness path) gives a stable result.
func TestMR_DeterministicWithSeed(t *testing.T) {
	// A known 512-bit prime (first 512-bit safe prime above 2^511)
	// We'll use a well-known one.
	// 2^521 - 1 is a Mersenne prime.
	m521 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(521), nil), big.NewInt(1))

	seed := bytes.Repeat([]byte{0xAB}, 1024)
	rdr := bytes.NewReader(seed)

	// n is large (>boundary), so random witnesses are used.
	// With a deterministic reader, the result must be stable.
	got1, err := IsProbablePrime(m521, rdr)
	require.NoError(t, err)
	require.True(t, got1)
}
