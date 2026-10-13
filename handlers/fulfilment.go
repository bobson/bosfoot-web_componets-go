package handlers

// Fulfilment values, stored on products.fulfilment (the model's mode) and on
// order_items.fulfilment (how that ordered line will actually be filled).
const (
	FulfilStock       = "stock"       // shipped from our shelf, stock decremented
	FulfilReservation = "reservation" // ordered from the brand, ~1–2 weeks
	FulfilDropship    = "dropship"    // ordered from the maker, ~7 days
)

// resolveLine decides how one order line is fulfilled, given the product's mode
// and whether the in-stock decrement for that exact size/colour succeeded.
//
//   - pre-launch (site.PreorderAll): every line is a reservation; stock is never
//     touched, so decremented is irrelevant.
//   - decremented: the pair is on our shelf → "stock", whatever the mode.
//   - not decremented (out of stock, or someone bought the last pair first):
//     reservation/dropship models still take the order → that mode; a "stock"
//     model (or an unknown mode) can't → ok=false (checkout shows sold out).
//
// It is pure so the decision table can be tested without a database.
func resolveLine(mode string, preorderAll, decremented bool) (fulfilment string, ok bool) {
	switch {
	case preorderAll:
		return FulfilReservation, true
	case decremented:
		return FulfilStock, true
	case mode == FulfilReservation || mode == FulfilDropship:
		return mode, true
	default:
		return "", false
	}
}
