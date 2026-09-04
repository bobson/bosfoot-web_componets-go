package handlers

import (
	"bosfoot/logger"
	"encoding/json"
	"net/http"
)

// TrackHandler records the one funnel step that isn't already a server request:
// add-to-cart. It lives entirely in the browser (localStorage + a dispatched
// event, no fetch), so unlike page views, the checkout page and the order POST —
// all of which are in the Caddy access log and slog — it would otherwise be
// invisible. Events are written to slog only (stdout + bosfoot.log), never the
// DB, so this public, unauthenticated beacon can't consume the small Aiven
// connection pool. Pixel events (ViewContent/InitiateCheckout/Lead) go straight
// to Meta client-side and are not sent here.
type TrackHandler struct {
	Logger *logger.Logger
}

// allowedEvents is a strict allowlist: the public endpoint can only append a
// known label to the log, never an arbitrary attacker-controlled string.
var allowedEvents = map[string]bool{"add_to_cart": true}

type trackReq struct {
	Event     string `json:"event"`
	ProductID int    `json:"product_id"`
	Size      int    `json:"size"`  // EU size, e.g. 42; 0 when the client omitted it
	Color     string `json:"color"` // colour name, e.g. "Chestnut"; "" when omitted
	Locale    string `json:"locale"`
}

// Track handles POST /api/cart-add (legacy alias /api/track). It is
// fire-and-forget: any malformed or
// disallowed beacon is silently dropped (204) rather than erroring, since the
// client uses navigator.sendBeacon and never reads the response.
func (h *TrackHandler) Track(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req trackReq
	// sendBeacon posts text/plain; we decode the body as JSON regardless. Cap it
	// so a flood of junk can't grow request memory.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil || !allowedEvents[req.Event] {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.Locale != "mk" && req.Locale != "sq" && req.Locale != "en" {
		req.Locale = ""
	}
	// Bound the size to the real EU range (37–48) so a junk beacon can't pollute
	// the log; anything out of range is dropped to 0 ("unknown"). Cap the colour
	// name length — it's an attacker-controlled free string in a public endpoint.
	if req.Size < 37 || req.Size > 48 {
		req.Size = 0
	}
	if len(req.Color) > 32 {
		req.Color = req.Color[:32]
	}
	h.Logger.Info("funnel", "event", req.Event, "product_id", req.ProductID,
		"size", req.Size, "color", req.Color, "locale", req.Locale, "ip", clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
