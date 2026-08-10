package site

import "testing"

// TestOrderNumberRoundTrip confirms ParseOrderNumber inverts OrderNumber, so the
// BF- number the owner sees in the listing/emails is exactly what the terminal
// tools accept back. Also checks a raw id (no offset) is rejected.
func TestOrderNumberRoundTrip(t *testing.T) {
	for _, id := range []int{1, 7, 64, 999, 12345} {
		s := OrderNumber(id)
		got, err := ParseOrderNumber(s)
		if err != nil || got != id {
			t.Errorf("ParseOrderNumber(OrderNumber(%d)=%q) = %d, %v; want %d, nil", id, s, got, err, id)
		}
	}
	// Accept a bare display number and a #-prefixed one.
	for _, s := range []string{"1064", "#1064", " bf-1064 "} {
		if got, err := ParseOrderNumber(s); err != nil || got != 64 {
			t.Errorf("ParseOrderNumber(%q) = %d, %v; want 64, nil", s, got, err)
		}
	}
	// A raw id (below the offset) and junk must error, not silently mis-map.
	for _, s := range []string{"64", "0", "BF-1000", "abc", ""} {
		if _, err := ParseOrderNumber(s); err == nil {
			t.Errorf("ParseOrderNumber(%q) should error", s)
		}
	}
}

// TestMKD pins the euro→denar conversion. The product display and the order
// handler both call site.MKD, so this is what a customer sees AND is charged.
func TestMKD(t *testing.T) {
	// Each want is euro*62 (floored to 10) + the flat DenarMarkup (200).
	cases := []struct {
		eur, want int
	}{
		{100, 6400},  // 100*62 = 6200 + 200
		{90, 5780},   // 90*62  = 5580 + 200
		{135, 8570},  // 135*62 = 8370 + 200
		{140, 8880},  // 140*62 = 8680 + 200
		{105, 6710},  // 105*62 = 6510 + 200
		{210, 13220}, // 210*62 = 13020 + 200
		{0, 200},     // 0*62   = 0    + 200
	}
	for _, c := range cases {
		if got := MKD(c.eur); got != c.want {
			t.Errorf("MKD(%d) = %d, want %d", c.eur, got, c.want)
		}
		if got := MKD(c.eur); got%10 != 0 {
			t.Errorf("MKD(%d) = %d does not end in 0", c.eur, got)
		}
	}
}

// TestEURDisplay pins the euro shown on sq/en to round(price_mkd/62) — the exact
// formula the `eur` template helper uses — so the guarded value matches what the
// site renders. With the flat DenarMarkup the euro no longer recovers the source
// euro (it rises ~3); this test locks in that intended, markup-aware relationship
// rather than the old round-trip.
func TestEURDisplay(t *testing.T) {
	for eur := 1; eur <= 500; eur++ {
		mkd := MKD(eur)
		want := int(float64(mkd)/MKDtoEUR + 0.5) // round(mkd/62)
		// The displayed euro is at least the source euro and drifts up a few from
		// the markup — never below the source, never absurdly high.
		if want < eur || want > eur+5 {
			t.Errorf("euro %d → mkd %d → shown €%d (want within [%d, %d])", eur, mkd, want, eur, eur+5)
		}
	}
}
