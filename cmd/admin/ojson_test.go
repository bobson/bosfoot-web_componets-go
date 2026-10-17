package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Every real data file must survive read → write unchanged, byte for byte, or
// admin edits would produce noisy diffs.
func TestOJSONRoundTripDataFiles(t *testing.T) {
	files, _ := filepath.Glob("../../data/*/catalog.json")
	more, _ := filepath.Glob("../../data/*/products/*.json")
	files = append(files, more...)
	if len(files) == 0 {
		t.Skip("no data files")
	}
	for _, f := range files {
		orig, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		o, err := parseOJSON(orig)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		got, err := formatOJSON(o)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !bytes.Equal(orig, got) {
			t.Errorf("%s changed on round trip", f)
		}
	}
}

func TestOJSONEdit(t *testing.T) {
	o, err := parseOJSON([]byte(`{"b":1,"a":{"x":"A & B"},"c":[1,2]}`))
	if err != nil {
		t.Fatal(err)
	}
	o.Set("d", true)
	o.Delete("b")
	o.Obj("a").Set("y", "ж")
	got, _ := formatOJSON(o)
	want := "{\n  \"a\": {\n    \"x\": \"A & B\",\n    \"y\": \"ж\"\n  },\n  \"c\": [\n    1,\n    2\n  ],\n  \"d\": true\n}\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
