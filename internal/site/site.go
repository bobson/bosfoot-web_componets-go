// Package site holds shop-wide configuration in one place, so values that were
// previously duplicated across Go, templates and client JS have a single home.
package site

import (
	"math"
	"os"
	"strconv"
	"strings"
)

// MetaPixelID returns the configured Meta (Facebook/Instagram) Pixel ID, or ""
// when unset. The analytics partial emits the pixel only when this is non-empty,
// so the pixel stays off until META_PIXEL_ID is added to .env — no code change
// needed to turn it on. Read per-call (cheap) so a restart isn't required.
func MetaPixelID() string {
	return strings.TrimSpace(os.Getenv("META_PIXEL_ID"))
}

// AssistantEnabled reports whether the Claude-backed free-text answering is
// configured (ANTHROPIC_API_KEY set). When false, the assistant widget still
// works but shows only the canned quick-questions — no text box, no API calls.
func AssistantEnabled() bool {
	return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
}

// PreorderAll forces the entire catalogue into preorder mode for the pre-launch
// phase: every product is shown and behaves as "preorder" — orderable without a
// size, with no inventory decrement — regardless of real stock. This is the
// temporary launch-soon layer; flip to false at launch to restore per-variant
// stock behaviour (in-stock = buy/decrement, OOS size = notify). A var (not
// const) so the guards aren't compiled out and it can be wired to env later.
var PreorderAll = false

// MKDtoEUR is the denar→euro conversion rate for the secondary EUR price shown
// on the sq/en locales. Single source of truth: the client JS reads it from a
// data-eur-rate attribute (injected via the `eurRate` template func) instead of
// hardcoding its own copy. Set to 62 so the denar price also covers inbound
// shipping (a ~1.6% uplift over the old 61).
const MKDtoEUR = 62.0

// EURRateString renders the rate for injection into a data attribute (e.g. "62").
func EURRateString() string {
	return strconv.FormatFloat(MKDtoEUR, 'f', -1, 64)
}

// FloorDenar rounds a denar price down to the nearest 10 so displayed prices
// always end in 0 (e.g. 5994 → 5990). Mirrored client-side in the cart/checkout
// JS so server- and browser-rendered prices match.
func FloorDenar(n int) int {
	return n / 10 * 10
}

// DenarMarkup is a flat surcharge (in denars) added to every product's derived
// denar price on top of the euro×rate conversion. It lets us raise all shoe
// prices by a fixed denar amount without touching the euro source or the rate.
// It's a multiple of 10 so prices still end in 0 after FloorDenar. Note this
// makes the euro shown on sq/en (round(price_mkd/62)) rise ~3 and no longer be a
// round source number — inherent to keeping the rate at 62 while adding a flat
// denar amount.
const DenarMarkup = 200

// MKD converts a euro price (the stored source) to denars: euro × rate, floored
// to the nearest 10, plus the flat DenarMarkup. This is the ONE conversion — the
// display handlers and the order handler both call it, so the price a customer
// sees equals the price charged. €135 → 8570; €100 → 6400.
func MKD(eur int) int {
	return FloorDenar(int(math.Round(float64(eur)*MKDtoEUR))) + DenarMarkup
}

// ShippingMKD is the flat delivery fee in denars; FreeShippingMKD is the goods
// subtotal at (or above) which delivery is free. Single source: injected into
// the checkout via data attributes (data-shipping-fee / data-free-shipping) and
// applied server-side in the order handler, so the fee shown equals the fee
// charged. The drawer note and the shipping policy page state these same numbers
// as prose — keep them in sync if the fee ever changes.
const (
	ShippingMKD     = 250
	FreeShippingMKD = 3500
)

// ShippingFor returns the delivery fee for a goods subtotal (denars): free once
// the subtotal reaches FreeShippingMKD, otherwise the flat ShippingMKD.
func ShippingFor(subtotal int) int {
	if subtotal >= FreeShippingMKD {
		return 0
	}
	return ShippingMKD
}
