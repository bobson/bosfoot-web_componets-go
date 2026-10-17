package main

import (
	"strings"
	"testing"

	"bosfoot/internal/notify"
)

func TestIntlPhone(t *testing.T) {
	cases := map[string]string{
		"070377075":     "+38970377075",
		"070 377 075":   "+38970377075",
		"+389 70326290": "+38970326290",
		"+38970255906":  "+38970255906",
		"0038970377075": "+38970377075",
		"38970377075":   "+38970377075",
		"070-377-075":   "+38970377075",
		"":              "",
		"   ":           "",
		"+":             "",
	}
	for in, want := range cases {
		if got := intlPhone(in); got != want {
			t.Errorf("intlPhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestViberMessage(t *testing.T) {
	one := []notify.ReviewLink{{ProductName: "Revive", URL: "https://bosfoot.com/mk/review/abc"}}
	m := viberMessage("Лендита", "mk", one)
	for _, want := range []string{"Здраво Лендита! Ова е Bosfoot", "твоите нови барефут обувки", "како ти одговара големината", "— Bosfoot", "https://bosfoot.com/mk/review/abc"} {
		if !strings.Contains(m, want) {
			t.Errorf("mk message missing %q:\n%s", want, m)
		}
	}
	if strings.Contains(m, "Revive:") {
		t.Errorf("single-product message should not label the link:\n%s", m)
	}
	two := append(one, notify.ReviewLink{ProductName: "Zing Fresh", URL: "https://bosfoot.com/mk/review/def"})
	if m := viberMessage("A", "mk", two); !strings.Contains(m, "Zing Fresh:\nhttps://bosfoot.com/mk/review/def") {
		t.Errorf("multi-product message should label each link:\n%s", m)
	}
	for _, loc := range []string{"sq", "en"} {
		if m := viberMessage("A", loc, one); !strings.Contains(m, "Zdravo A! Ova e Bosfoot") || !strings.Contains(m, "Blagodaram! — Bosfoot") {
			t.Errorf("%s should get Latin-script Macedonian:\n%s", loc, m)
		}
	}
}

func TestParseIDs(t *testing.T) {
	got, err := parseIDs("64, 65,67 ,81")
	if err != nil || len(got) != 4 || got[0] != 64 || got[3] != 81 {
		t.Fatalf("parseIDs = %v, %v", got, err)
	}
	if got, err := parseIDs(""); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v, %v", got, err)
	}
	if _, err := parseIDs("64,abc"); err == nil {
		t.Fatal("want error for non-numeric id")
	}
}
