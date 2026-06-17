// Package locale handles locale extraction from URL paths and UI string lookup.
package locale

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// LoadUI builds the UI string table from two co-existing sources, so pages can
// be migrated to the per-page layout incrementally:
//
//   - Flat per-locale files {dir}/mk.json, sq.json, en.json — one big file per
//     language, shaped {"key": "value"}. The original layout.
//   - Per-page / shared files {dir}/pages/*.json and {dir}/common.json — each
//     shaped {"key": {"mk":…, "sq":…, "en":…}}, so all three languages of a
//     string live together. Easier to add/remove a page's text in one place.
//
// Both feed the same lookup, so T(loc, key) and {{t}} are unchanged regardless
// of where a string lives. Per-page values override flat ones on key collision.
func LoadUI(dir string) (*UI, error) {
	ui := &UI{strings: make(map[string]map[string]string)}
	for _, loc := range All {
		ui.strings[loc] = make(map[string]string)
		// Flat per-locale files are optional (the strings now live in the
		// grouped common.json + pages/*.json). Load them if present, skip if not.
		path := fmt.Sprintf("%s/%s.json", dir, loc)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		m := make(map[string]string)
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		ui.strings[loc] = m
	}
	if err := ui.loadGrouped(dir); err != nil {
		return nil, err
	}
	return ui, nil
}

// loadGrouped merges the language-grouped files ({dir}/common.json and
// {dir}/pages/*.json) into the flat lookup. Each file maps a full key to its
// per-locale values: {"contact.title": {"mk":…, "sq":…, "en":…}}.
func (ui *UI) loadGrouped(dir string) error {
	patterns := []string{
		filepath.Join(dir, "common.json"),
		filepath.Join(dir, "pages", "*.json"),
	}
	for _, pattern := range patterns {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			var grouped map[string]map[string]string // key -> locale -> value
			if err := json.Unmarshal(data, &grouped); err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			for key, byLoc := range grouped {
				for loc, val := range byLoc {
					if ui.strings[loc] == nil {
						ui.strings[loc] = make(map[string]string)
					}
					ui.strings[loc][key] = val
				}
			}
		}
	}
	return nil
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
