package main

import "testing"

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
