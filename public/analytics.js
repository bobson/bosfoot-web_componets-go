// Funnel tracking — two sinks, deliberately minimal:
//
//  - Meta Pixel: fires the standard e-commerce events for ad optimisation and
//    retargeting, but ONLY if the pixel actually loaded. The pixel is gated
//    server-side on META_PIXEL_ID, so when it's unset window.fbq is undefined
//    and every call here no-ops.
//
//  - First-party beacon: only add-to-cart is sent to /api/track, because it's
//    the single funnel step with no server request of its own. Page views, the
//    checkout page and the order POST are already in the access log + slog, so
//    beaconing them too would just duplicate data and add load.
//
// Call sites pass Meta's event name (ViewContent, AddToCart, InitiateCheckout,
// Lead); the beacon maps add-to-cart to the snake_case label the server allows.
export function track(event, data = {}) {
  if (window.fbq) window.fbq('track', event, data);

  if (event === 'AddToCart' && navigator.sendBeacon) {
    const body = JSON.stringify({
      event: 'add_to_cart',
      product_id: Number(data.product_id) || 0,
      locale: document.documentElement.lang || '',
    });
    navigator.sendBeacon('/api/track', body);
  }
}
