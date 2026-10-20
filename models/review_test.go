package models

import "testing"

func TestReviewDisplayName(t *testing.T) {
	cases := []struct{ author, last, want string }{
		{"Александар", "Илиоски", "Александар Илиоски"},
		{"Aleksandar", "bozinovski", "Aleksandar Bozinovski"},
		{"  Pance ", "Zafirov", "Pance Zafirov"},
		{"Ана Петрова", "Петрова", "Ана Петрова"}, // already gave a surname
		{"Lendita", "", "Lendita"},                // order deleted / unknown
	}
	for _, c := range cases {
		if got := ReviewDisplayName(c.author, c.last); got != c.want {
			t.Errorf("ReviewDisplayName(%q, %q) = %q, want %q", c.author, c.last, got, c.want)
		}
	}
}
