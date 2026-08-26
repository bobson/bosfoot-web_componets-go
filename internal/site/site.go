// Package site holds shop-wide configuration in one place, so values that were
// previously duplicated across Go, templates and client JS have a single home.
package site

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// OrderNumberOffset is added to an order's DB id to produce the customer-facing
// number, so early orders don't read as "#1" and reveal low volume. Display-only
// — the integer id stays the source of truth (order_items, review_tokens etc.
// still reference it).
const OrderNumberOffset = 1000

// OrderNumber renders an order id as the branded, phone-friendly number shown to
// customers and in the terminal tools, e.g. 64 → "BF-1064".
func OrderNumber(id int) string {
	return fmt.Sprintf("BF-%d", id+OrderNumberOffset)
}

// ParseOrderNumber is the inverse of OrderNumber: it turns the customer-facing
// number back into the DB id, so the terminal tools can accept the same "BF-1064"
// the owner sees everywhere. It accepts "BF-1064", "bf-1064", "#1064" or a bare
// "1064" (all the display number), and errors if the result isn't a real id (≥1),
// which is what a raw id like "64" would map to — steering the owner to the BF form.
func ParseOrderNumber(s string) (int, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(strings.ToUpper(t), "BF-")
	t = strings.TrimPrefix(t, "#")
	n, err := strconv.Atoi(strings.TrimSpace(t))
	if err != nil {
		return 0, fmt.Errorf("not an order number %q (use the BF- number, e.g. BF-1064)", s)
	}
	id := n - OrderNumberOffset
	if id < 1 {
		return 0, fmt.Errorf("no such order %q (use the BF- number you see, e.g. BF-1064)", s)
	}
	return id, nil
}

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
// It's a multiple of 10 so prices still end in 0 after FloorDenar.
//
// Currently 0: reverted 2026-08-26 back to pure euro×62 after a 2-week sales
// stall that began when the markup pushed every price up 200 denars (e.g. €100
// went 6200 → 6400). With the markup at 0 the euro shown on sq/en
// (round(price_mkd/62)) again recovers the round source euro. Raise this again
// only if a fixed across-the-board denar uplift is wanted.
const DenarMarkup = 0

// MKD converts a euro price (the stored source) to denars: euro × rate, floored
// to the nearest 10, plus the flat DenarMarkup. This is the ONE conversion — the
// display handlers and the order handler both call it, so the price a customer
// sees equals the price charged. €135 → 8370; €100 → 6200.
func MKD(eur int) int {
	return FloorDenar(int(math.Round(float64(eur)*MKDtoEUR))) + DenarMarkup
}

// ClearancePct is a site-wide clearance markdown applied to EVERY product's price
// (0 = no sale). When > 0 the full price is shown struck through and the
// discounted price is both displayed and charged. It layers on top of MKD (the
// canonical full price) rather than being baked in, so a running clearance always
// has a "was" price to strike, and ending it is a one-constant flip back to 0.
// 2026-08-26: 0.10 — summer clearance across the whole catalogue.
const ClearancePct = 0.10

// SaleActive reports whether a site-wide clearance is currently running.
func SaleActive() bool { return ClearancePct > 0 }

// SalePrice applies ClearancePct to a full denar price, floored to the nearest 10
// so sale prices still end in 0 like every other price. With the clearance off it
// returns the price unchanged. The display handlers and the order handler both
// call it, so the price a customer sees equals the price charged.
func SalePrice(full int) int {
	if ClearancePct <= 0 {
		return full
	}
	return FloorDenar(int(math.Round(float64(full) * (1 - ClearancePct))))
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
