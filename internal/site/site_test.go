package site

import "testing"

// TestMKD pins the euro→denar conversion. The product display and the order
// handler both call site.MKD, so this is what a customer sees AND is charged.
func TestMKD(t *testing.T) {
	cases := []struct {
		eur, want int
	}{
		{100, 6200},  // 100*62 = 6200
		{90, 5580},   // 90*62  = 5580
		{135, 8370},  // 135*62 = 8370
		{140, 8680},  // 140*62 = 8680
		{105, 6510},  // 105*62 = 6510
		{210, 13020}, // 210*62 = 13020
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

// TestEURRecovers confirms the displayed euro (round(mkd/62)) recovers the exact
// source euro, so showing the whole-number EUR matches the price you set.
func TestEURRecovers(t *testing.T) {
	for eur := 1; eur <= 500; eur++ {
		mkd := MKD(eur)
		if back := int(float64(mkd)/MKDtoEUR + 0.5); back != eur {
			t.Errorf("euro %d → mkd %d → back %d (should round-trip)", eur, mkd, back)
		}
	}
}
