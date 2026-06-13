import { initNavDrawer } from "./components/nav-drawer.js";
import { initCartDrawer } from "./components/cart-drawer.js";
import { initNavLocale } from "./components/nav-locale.js";
import { initNavSearch } from "./components/nav-search.js";
import { initCheckoutForm } from "./components/checkout-form.js";
import { initListingFilter } from "./components/listing-filter.js";
import { initProductDetail } from "./components/product-detail.js";
import { initScrollReveal } from "./components/scroll-reveal.js";

(function () {
  "use strict";

  // Component Registration for better error handling and potential future features like hot reloading.
  const components = [
    initNavDrawer,
    initCartDrawer,
    initNavLocale,
    initNavSearch,
    initCheckoutForm,
    initListingFilter,
    initProductDetail,
    initScrollReveal,
  ];

  components.forEach((init) => {
    try {
      init();
    } catch (err) {
      console.error("Component initialization failed", err);
    }
  });

  // Global state logic...
  function getCartCount() {
    try {
      const cart = JSON.parse(localStorage.getItem("bosfoot_cart") || "[]");
      return cart.reduce((n, item) => n + (item.qty || 0), 0);
    } catch {
      return 0;
    }
  }

  function updateCartBadge() {
    const btn = document.getElementById("cart-btn");
    if (!btn) return;

    let badge = btn.querySelector(".cart-badge");
    if (!badge) {
      badge = document.createElement("span");
      badge.className = "cart-badge";
      badge.setAttribute("aria-hidden", "true");
      btn.appendChild(badge);
    }

    const count = getCartCount();
    badge.textContent = count;
    badge.hidden = count === 0;
  }

  updateCartBadge();

  // Keep badge in sync: another tab (storage) or this tab (cart:updated,
  // dispatched by the product page and the cart drawer on any change).
  window.addEventListener("storage", (e) => {
    if (e.key === "bosfoot_cart") updateCartBadge();
  });
  window.addEventListener("cart:updated", updateCartBadge);
})();
