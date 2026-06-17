package site

import "testing"

// TestMKD pins the euro→denar conversion. The product display and the order
// handler both call site.MKD, so this is what a customer sees AND is charged.
func TestMKD(t *testing.T) {
	cases := []struct {
		eur, want int
	}{
		{100, 6100},  // 100*61 = 6100
		{90, 5490},   // 90*61  = 5490
		{135, 8230},  // 135*61 = 8235 → floor to 8230
		{140, 8540},  // 140*61 = 8540
		{105, 6400},  // 105*61 = 6405 → floor to 6400
		{210, 12810}, // 210*61 = 12810
		{0, 0},
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

// TestEURRecovers confirms the displayed euro (round(mkd/61)) recovers the exact
// source euro, so showing the whole-number EUR matches the price you set.
func TestEURRecovers(t *testing.T) {
	for eur := 1; eur <= 500; eur++ {
		mkd := MKD(eur)
		if back := int(float64(mkd)/MKDtoEUR + 0.5); back != eur {
			t.Errorf("euro %d → mkd %d → back %d (should round-trip)", eur, mkd, back)
		}
	}
}
