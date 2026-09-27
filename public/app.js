// Cache-busting: every JS module is versioned by its OWN content hash through
// the import map the server emits before this script (Renderer.importMap), so
// the plain paths below resolve to /components/x.js?v=<hash>. Don't append a
// version here — a URL that isn't a map key would bypass the map.
import { bootstrapPixel } from './prefs.js';

// Install the Meta Pixel for every visitor. The fbq stub + init + PageView run
// synchronously here (before the dynamic component imports below), so a
// ViewContent fired at product-detail init is queued instead of racing an
// undefined window.fbq — but the heavy fbevents.js library is deferred to idle
// inside bootstrapPixel(), keeping it out of the LCP window. No-op when
// META_PIXEL_ID is unset (no <meta> tag). The prefs-ui component shows an
// informational cookie notice — it does NOT gate the pixel.
bootstrapPixel();

// Each component is loaded with a dynamic import and initialised in isolation.
// Static `import` would force the browser to parse EVERY component before app.js
// runs, so one broken/unfinished module takes down all interactivity. Dynamic
// import + try/catch contains a failure to that single component (logged), and a
// finished module just lights up — no edits here needed.
const components = [
  ['./components/nav-drawer.js', 'initNavDrawer'],
  ['./components/cart-drawer.js', 'initCartDrawer'],
  ['./components/nav-locale.js', 'initNavLocale'],
  ['./components/nav-search.js', 'initNavSearch'],
  ['./components/checkout-form.js', 'initCheckoutForm'],
  ['./components/review-form.js', 'initReviewForm'],
  ['./components/listing-filter.js', 'initListingFilter'],
  ['./components/product-detail.js', 'initProductDetail'],
  ['./components/size-guide-modal.js', 'initSizeGuideModal'],
  ['./components/foot-finder.js', 'initFootFinder'],
  ['./components/scroll-reveal.js', 'initScrollReveal'],
  ['./components/prefs-ui.js', 'initPrefsUI'],
  ['./components/assistant.js', 'initAssistant'],
];

for (const [path, fn] of components) {
  import(path)
    .then((mod) => mod[fn]?.())
    .catch((err) => console.error(`Component failed to load: ${path}`, err));
}

// Cart badge — runs synchronously, independent of the components above.
function getCartCount() {
  try {
    const cart = JSON.parse(localStorage.getItem('bosfoot_cart') || '[]');
    return cart.reduce((n, item) => n + (item.qty || 0), 0);
  } catch {
    return 0;
  }
}

function updateCartBadge() {
  const btn = document.getElementById('cart-btn');
  if (!btn) return;

  let badge = btn.querySelector('.cart-badge');
  if (!badge) {
    badge = document.createElement('span');
    badge.className = 'cart-badge';
    badge.setAttribute('aria-hidden', 'true');
    btn.appendChild(badge);
  }

  const count = getCartCount();
  badge.textContent = count;
  badge.hidden = count === 0;
}

updateCartBadge();

window.addEventListener('storage', (e) => {
  if (e.key === 'bosfoot_cart') updateCartBadge();
});
window.addEventListener('cart:updated', updateCartBadge);
