package locale

import (
	"testing"
)

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
