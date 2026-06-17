package locale

import (
	"os"
	"testing"
)

// TestPublicLocalesInSync loads the real locale files (flat + per-page) and
// fails if any key is missing from a language — the drift guard for the mixed
// flat/grouped layout. Skips if run somewhere the files aren't reachable.
func TestPublicLocalesInSync(t *testing.T) {
	const dir = "../../public/locales"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("public locales not reachable from here")
	}
	ui, err := LoadUI(dir)
	if err != nil {
		t.Fatalf("LoadUI(%s) failed: %v", dir, err)
	}

	// Collect every key seen in any locale, then assert each locale has it.
	allKeys := map[string]bool{}
	for _, loc := range All {
		for k := range ui.strings[loc] {
			allKeys[k] = true
		}
	}
	for key := range allKeys {
		for _, loc := range All {
			if _, ok := ui.strings[loc][key]; !ok {
				t.Errorf("key %q missing for locale %q", key, loc)
			}
		}
	}

	// Sanity: a key that now lives in pages/contact.json resolves per-locale.
	if got := ui.T("en", "contact.title"); got == "contact.title" || got == "" {
		t.Errorf("grouped key contact.title did not load (got %q)", got)
	}
}

func TestFromPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"mk", "mk"},
		{"sq", "sq"},
		{"en", "en"},
		{"invalid", "mk"},
		{"", "mk"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := FromPath(tt.input); got != tt.want {
				t.Errorf("FromPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"mk", true},
		{"sq", true},
		{"en", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsValid(tt.input); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadUIAndT(t *testing.T) {
	ui, err := LoadUI("testdata")
	if err != nil {
		t.Fatalf("LoadUI failed: %v", err)
	}

	tests := []struct {
		loc, key, want string
	}{
		{"mk", "test_key", "MK translation"},
		{"sq", "test_key", "SQ translation"},
		{"en", "test_key", "EN translation"},
		{"en", "missing_key", "missing_key"}, // Should return key if missing
		{"invalid", "test_key", "MK translation"}, // Should fall back to MK
	}
	for _, tt := range tests {
		t.Run(tt.loc+"_"+tt.key, func(t *testing.T) {
			if got := ui.T(tt.loc, tt.key); got != tt.want {
				t.Errorf("UI.T() = %v, want %v", got, tt.want)
			}
		})
	}
}
