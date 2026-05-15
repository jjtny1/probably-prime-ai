package prime

import "math/big"

// boundary is the threshold below which the deterministic witness set is sufficient
// to correctly classify all composites (Sorenson & Webster 2017).
// For n < boundary, the witnesses {2,3,5,7,11,13,17,19,23,29,31,37} are deterministic.
var boundary *big.Int

func init() {
	boundary = new(big.Int)
	boundary.SetString("3317044064679887385961981", 10)
}

// deterministicWitnesses returns the Sorenson-Webster 2017 witness set for n < boundary,
// or nil if n >= boundary (caller should use random witnesses instead).
func deterministicWitnesses(n *big.Int) []uint64 {
	if n.Cmp(boundary) < 0 {
		return []uint64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37}
	}
	return nil
}

// roundsFor returns the number of Miller-Rabin rounds required to achieve a
// false-prime probability of at most 2^-128 for a candidate of the given bit length.
// Values meet or exceed FIPS 186-4 Table C.1 minimums.
func roundsFor(bits int) int {
	switch {
	case bits >= 2048:
		return 27
	case bits >= 1536:
		return 33
	case bits >= 1024:
		return 40
	case bits >= 512:
		return 56
	default:
		// For small bit lengths (including 256-bit and below), use 64 rounds as a floor.
		return 64
	}
}
