// Package locale handles locale extraction from URL paths and UI string lookup.
package locale

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	MK      = "mk"
	SQ      = "sq"
	EN      = "en"
	Default = MK
)

var All = []string{MK, SQ, EN}

// UI holds translated strings for all locales, loaded once at startup.
type UI struct {
	strings map[string]map[string]string
}

// LoadUI reads {dir}/mk.json, sq.json, en.json and returns a UI ready to use.
func LoadUI(dir string) (*UI, error) {
	ui := &UI{strings: make(map[string]map[string]string)}
	for _, loc := range All {
		path := fmt.Sprintf("%s/%s.json", dir, loc)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		m := make(map[string]string)
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		ui.strings[loc] = m
	}
	return ui, nil
}

// T returns the translated string for loc and key.
// Falls back to MK, then returns the key itself so missing strings are visible.
func (ui *UI) T(loc, key string) string {
	if m, ok := ui.strings[loc]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if m, ok := ui.strings[MK]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}

// FromPath extracts and validates the locale from a URL path segment.
// Returns Default if the segment is not a supported locale code.
func FromPath(segment string) string {
	switch segment {
	case MK, SQ, EN:
		return segment
	}
	return Default
}

// IsValid reports whether loc is a supported locale code.
func IsValid(loc string) bool {
	return loc == MK || loc == SQ || loc == EN
}
