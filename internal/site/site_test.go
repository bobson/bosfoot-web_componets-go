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
	// Each want is euro*62 (floored to 10) + the flat DenarMarkup (currently 0).
	cases := []struct {
		eur, want int
	}{
		{100, 6200},  // 100*62 = 6200 + 0
		{90, 5580},   // 90*62  = 5580 + 0
		{135, 8370},  // 135*62 = 8370 + 0
		{140, 8680},  // 140*62 = 8680 + 0
		{105, 6510},  // 105*62 = 6510 + 0
		{210, 13020}, // 210*62 = 13020 + 0
		{0, 0},       // 0*62   = 0    + 0
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

// TestSalePrice pins the site-wide clearance markdown. With ClearancePct 0.10 the
// full price drops 10%, floored to the nearest 10 so it still ends in 0.
func TestSalePrice(t *testing.T) {
	cases := []struct {
		full, want int
	}{
		{6200, 5580},  // 6200*0.9 = 5580
		{8370, 7530},  // 8370*0.9 = 7533 → floor 7530
		{6510, 5850},  // 6510*0.9 = 5859 → floor 5850
		{13020, 11710}, // 13020*0.9 = 11718 → floor 11710
		{0, 0},
	}
	for _, c := range cases {
		if got := SalePrice(c.full); got != c.want {
			t.Errorf("SalePrice(%d) = %d, want %d", c.full, got, c.want)
		}
		if got := SalePrice(c.full); got%10 != 0 {
			t.Errorf("SalePrice(%d) = %d does not end in 0", c.full, got)
		}
	}
	if !SaleActive() {
		t.Error("SaleActive() = false, want true while ClearancePct > 0")
	}
}

// TestEURDisplay pins the euro shown on sq/en to round(price_mkd/62) — the exact
// formula the `eur` template helper uses — so the guarded value matches what the
// site renders. With DenarMarkup at 0 the euro again recovers the source euro
// exactly; the range guard still allows a few denars of drift so the test keeps
// passing if a small markup is reintroduced.
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
