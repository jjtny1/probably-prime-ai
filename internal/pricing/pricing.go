// Package pricing maps prime bit sizes to USD price strings for the x402 payment layer.
package pricing

// tiers maps allowed bit sizes to their USD price strings.
var tiers = map[int]string{
	256:  "$0.001",
	512:  "$0.003",
	1024: "$0.01",
	2048: "$0.05",
}

// allowedBits is the canonical sorted list of allowed bit sizes.
var allowedBits = []int{256, 512, 1024, 2048}

// Tier returns the USD price string and true for a recognised bit size,
// or ("", false) for any unrecognised size.
func Tier(bits int) (string, bool) {
	price, ok := tiers[bits]
	return price, ok
}

// AllowedBits returns a fresh copy of the sorted list of supported bit sizes.
// Callers may freely mutate the returned slice; it does not affect package state.
func AllowedBits() []int {
	cp := make([]int, len(allowedBits))
	copy(cp, allowedBits)
	return cp
}
