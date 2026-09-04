// Funnel tracking — two sinks, deliberately minimal:
//
//  - Meta Pixel: fires the standard e-commerce events for ad optimisation and
//    retargeting, but ONLY if the pixel actually loaded. The pixel is gated
//    server-side on META_PIXEL_ID, so when it's unset window.fbq is undefined
//    and every call here no-ops.
//
//  - First-party beacon: only add-to-cart is sent to /api/cart-add, because it's
//    the single funnel step with no server request of its own. Page views, the
//    checkout page and the order POST are already in the access log + slog, so
//    beaconing them too would just duplicate data and add load.
//
// Call sites pass Meta's event name (ViewContent, AddToCart, InitiateCheckout,
// Lead); the beacon maps add-to-cart to the snake_case label the server allows.
export function track(event, data = {}) {
  if (window.fbq) window.fbq('track', event, data);

  // First-party beacon is deliberately NOT gated on cookie consent: it sets no
  // cookie and stores no identifier (only product_id + size + colour + locale, all
  // product attributes), so it's anonymous legitimate-interest analytics, not
  // marketing tracking. The Meta Pixel above is gated only on META_PIXEL_ID
  // (server-side): when it's unset window.fbq stays undefined and the call
  // no-ops. The cookie notice (public/prefs.js) is informational — it does NOT
  // hold the pixel until the visitor accepts.
  if (event === 'AddToCart' && navigator.sendBeacon) {
    const body = JSON.stringify({
      event: 'add_to_cart',
      product_id: Number(data.product_id) || 0,
      // size + colour make the beacon a size-level demand signal (which
      // size/colour gets added but not bought) — the reorder question. Both are
      // optional; the server bounds/normalises them.
      size: Number(data.size) || 0,
      color: (data.color || '').toString(),
      locale: document.documentElement.lang || '',
    });
    // Path is "cart-add", NOT "track": privacy blockers drop any /api/track,
    // which was silently killing ~96% of these beacons.
    navigator.sendBeacon('/api/cart-add', body);
  }
}
