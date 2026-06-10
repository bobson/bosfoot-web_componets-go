(function () {
  'use strict';

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

  // Keep badge in sync if cart changes in another tab.
  window.addEventListener('storage', (e) => {
    if (e.key === 'bosfoot_cart') updateCartBadge();
  });
}());
