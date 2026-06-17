// Package site holds shop-wide configuration in one place, so values that were
// previously duplicated across Go, templates and client JS have a single home.
package site

import (
	"math"
	"strconv"
)

// MKDtoEUR is the denar→euro conversion rate for the secondary EUR price shown
// on the sq/en locales. Single source of truth: the client JS reads it from a
// data-eur-rate attribute (injected via the `eurRate` template func) instead of
// hardcoding its own copy.
const MKDtoEUR = 61.0

// EURRateString renders the rate for injection into a data attribute (e.g. "61").
func EURRateString() string {
	return strconv.FormatFloat(MKDtoEUR, 'f', -1, 64)
}

// FloorDenar rounds a denar price down to the nearest 10 so displayed prices
// always end in 0 (e.g. 5994 → 5990). Mirrored client-side in the cart/checkout
// JS so server- and browser-rendered prices match.
func FloorDenar(n int) int {
	return n / 10 * 10
}

// MKD converts a euro price (the stored source) to denars: euro × rate, floored
// to the nearest 10. This is the ONE conversion — the template `mkd` helper and
// the order handler both call it, so the price a customer sees equals the price
// charged. €135 → 8235 → 8230; €100 → 6100.
func MKD(eur int) int {
	return FloorDenar(int(math.Round(float64(eur) * MKDtoEUR)))
}
