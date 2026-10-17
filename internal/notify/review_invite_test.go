package notify

import (
	"strings"
	"testing"
)

// The review invite is always Macedonian: Cyrillic for /mk orders, Latin
// script for /sq and /en orders.
func TestReviewInviteScript(t *testing.T) {
	d := ReviewInviteData{Email: "a@b.mk", Name: "Taxhedin Q", Links: []ReviewLink{{ProductName: "Feldom 3", URL: "https://x/review/t"}}}

	d.Locale = "mk"
	if m := string(buildReviewInviteMessage("f@x", "", d)); !strings.Contains(m, "Здраво Taxhedin,") || !strings.Contains(m, "Оцени: Feldom 3") {
		t.Errorf("mk should be Cyrillic Macedonian:\n%s", m)
	}
	for _, loc := range []string{"sq", "en"} {
		d.Locale = loc
		m := string(buildReviewInviteMessage("f@x", "", d))
		if !strings.Contains(m, "Zdravo Taxhedin,") || !strings.Contains(m, "Oceni: Feldom 3") || !strings.Contains(m, "Timot na Bosfoot") {
			t.Errorf("%s should be Latin-script Macedonian:\n%s", loc, m)
		}
	}
}
