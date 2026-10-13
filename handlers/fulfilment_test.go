package handlers

import "testing"

func TestResolveLine(t *testing.T) {
	cases := []struct {
		name        string
		mode        string
		preorderAll bool
		decremented bool
		want        string
		wantOK      bool
	}{
		{"stock model, size in stock", "stock", false, true, FulfilStock, true},
		{"stock model, size sold out", "stock", false, false, "", false},
		// Two buyers race for the last pair: the loser's decrement matches 0 rows.
		{"stock model, lost the last-pair race", "stock", false, false, "", false},
		{"dropship model, lost the last-pair race → maker ships it", "dropship", false, false, FulfilDropship, true},
		{"dropship model, size on our shelf", "dropship", false, true, FulfilStock, true},
		// qty 2 ordered with 1 on the shelf: the `qty >= 2` decrement fails, so the
		// whole line goes to the supplier and the shelf pair stays for someone else.
		{"reservation model, qty exceeds shelf", "reservation", false, false, FulfilReservation, true},
		{"reservation model, size on our shelf", "reservation", false, true, FulfilStock, true},
		{"unknown mode is treated as stock", "", false, false, "", false},
		{"pre-launch: everything is a reservation", "stock", true, false, FulfilReservation, true},
		{"pre-launch ignores a decrement flag", "dropship", true, true, FulfilReservation, true},
	}
	for _, c := range cases {
		got, ok := resolveLine(c.mode, c.preorderAll, c.decremented)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.wantOK)
		}
	}
}
