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
  ['./components/listing-filter.js', 'initListingFilter'],
  ['./components/product-detail.js', 'initProductDetail'],
  ['./components/size-guide-modal.js', 'initSizeGuideModal'],
  ['./components/scroll-reveal.js', 'initScrollReveal'],
];

// Cache-bust component imports with app.js's own asset version (its ?v=hash).
// app.js is loaded versioned ({{asset "/app.js"}}), but these dynamic imports
// used bare paths, so aggressive caches (notably the Facebook in-app browser,
// which ignores hard-refresh) could pair a fresh deploy's HTML with stale
// cached component JS — e.g. old checkout JS reading a renamed form field.
// Stamping app.js's version onto each import forces a fresh fetch on deploy.
const VER = new URL(import.meta.url).search; // "?v=abcd1234" (or "" in dev)

for (const [path, fn] of components) {
  import(path + VER)
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
