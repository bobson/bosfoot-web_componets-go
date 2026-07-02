// <checkout-form> — an HTML Web Component (custom element, light DOM, no shadow).
// Renders the order summary from the localStorage cart, validates the form,
// POSTs to /api/orders (which re-prices server-side), then swaps in the
// confirmation panel. Global CSS (checkout.css) styles it normally.

import { track } from '../funnel.js';

const CART_KEY = 'bosfoot_cart';
// Default rate; overridden at connect from the data-eur-rate attribute, which
// the server injects from internal/site (the single source of truth).
let MKD_TO_EUR = 61;

function floorDenar(n) {
  // round down to the nearest 10 so prices end in 0 — mirrors site.FloorDenar.
  return Math.floor(n / 10) * 10;
}
function fmtMKD(n) {
  return String(floorDenar(n)).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}
function fmtEUR(n) {
  // whole number — EUR is the round source price, MKD is the converted value
  return String(Math.round(floorDenar(n) / MKD_TO_EUR));
}
function esc(s) {
  const d = document.createElement('div');
  d.textContent = s == null ? '' : String(s);
  return d.innerHTML;
}

class CheckoutForm extends HTMLElement {
  connectedCallback() {
    this.currency = this.dataset.currency || 'MKD';
    this.showEur = this.dataset.showEur === '1';
    MKD_TO_EUR = parseFloat(this.dataset.eurRate) || MKD_TO_EUR;
    this.labelSize = this.dataset.labelSize || 'Size';
    this.labelPreorder = this.dataset.labelPreorder || 'Preorder';
    this.labelEmailSent = this.dataset.labelEmailSent || '';
    this.labelSpamNote = this.dataset.labelSpamNote || '';
    this.errorGeneric = this.dataset.errorGeneric || 'Something went wrong. Please try again.';
    this.errorStock =
      this.dataset.errorStock || 'Some items just sold out. Please review your cart.';

    this.emptyEl = this.querySelector('[data-checkout-empty]');
    this.gridEl = this.querySelector('[data-checkout-grid]');
    this.form = this.querySelector('[data-checkout-form]');
    this.itemsEl = this.querySelector('[data-summary-items]');
    this.preorderEl = this.querySelector('[data-summary-preorder]');
    this.subtotalEl = this.querySelector('[data-summary-subtotal]');
    this.totalEl = this.querySelector('[data-summary-total]');
    this.errorEl = this.querySelector('[data-checkout-error]');
    this.submitBtn = this.querySelector('[data-checkout-submit]');
    this.confirmEl = this.querySelector('[data-checkout-confirm]');
    this.confirmOrderEl = this.querySelector('[data-confirm-order]');
    this.confirmEmailNoteEl = this.querySelector('[data-confirm-email-note]');
    this.confirmCodEl = this.querySelector('[data-confirm-cod]');
    this.confirmBankEl = this.querySelector('[data-confirm-bank]');

    this.form.addEventListener('submit', (e) => this.onSubmit(e));

    // Re-render if the cart changes in another tab while checkout is open.
    window.addEventListener('storage', (e) => {
      if (e.key === CART_KEY) this.renderSummary();
    });

    this.renderSummary();

    // Funnel: reached checkout with items (pixel-only; the GET is in the access
    // log). Skip when the cart is empty or the confirmation panel is showing.
    if (this.confirmEl.hidden && this.read().length) track('InitiateCheckout');
  }

  read() {
    try {
      const cart = JSON.parse(localStorage.getItem(CART_KEY) || '[]');
      // Self-healing: fix any old URLs with spaces that were broken by the cleanup
      let changed = false;
      cart.forEach((item) => {
        if (item.imageUrl && item.imageUrl.includes(' ')) {
          item.imageUrl = item.imageUrl.replace(/\s+/g, '-');
          changed = true;
        }
      });
      if (changed) {
        localStorage.setItem(CART_KEY, JSON.stringify(cart));
      }
      return cart;
    } catch {
      return [];
    }
  }

  price(n) {
    let s = `${fmtMKD(n)} ${this.currency}`;
    if (this.showEur) s += ` · ${fmtEUR(n)} €`;
    return s;
  }

  renderSummary() {
    const cart = this.read();

    // Empty cart: hide the form/summary, show the empty state. (Skip while the
    // confirmation panel is up — the cart was just cleared on success.)
    if (!cart.length) {
      if (this.confirmEl.hidden) {
        this.gridEl.hidden = true;
        this.emptyEl.hidden = false;
      }
      return;
    }

    this.emptyEl.hidden = true;
    this.gridEl.hidden = false;

    this.itemsEl.innerHTML = cart.map((i) => this.row(i)).join('');
    if (this.preorderEl) this.preorderEl.hidden = !cart.some((i) => i.preorder);
    const subtotal = cart.reduce((s, i) => s + i.price * i.qty, 0);
    this.subtotalEl.textContent = this.price(subtotal);
    this.totalEl.textContent = this.price(subtotal);
  }

  row(i) {
    const img = i.imageUrl
      ? `<img class="checkout-item__img" src="${esc(i.imageUrl)}" alt="${esc(i.productName)}" loading="lazy">`
      : `<div class="checkout-item__img"></div>`;
    return `
      <article class="checkout-item">
        <div class="checkout-item__media">
          ${img}
          <span class="checkout-item__qty">${esc(i.qty)}</span>
        </div>
        <div class="checkout-item__info">
          <p class="checkout-item__name font-medium">${esc(i.productName)}</p>
          <p class="checkout-item__variant text-xs text-muted">${
            i.preorder
              ? `<span class="checkout-item__preorder">${esc(this.labelPreorder)}</span> · ${esc(i.color)}`
              : `${esc(this.labelSize)} ${esc(i.size)} · ${esc(i.color)}`
          }</p>
        </div>
        <span class="checkout-item__price text-sm">${esc(this.price(i.price * i.qty))}</span>
      </article>`;
  }

  async onSubmit(e) {
    e.preventDefault();
    this.errorEl.hidden = true;

    const cart = this.read();
    if (!cart.length) {
      this.renderSummary();
      return;
    }
    // Let the browser surface native validation messages for required fields.
    if (!this.form.reportValidity()) return;

    const fd = new FormData(this.form);
    const payload = {
      email: (fd.get('email') || '').trim(),
      phone: (fd.get('phone') || '').trim(),
      first_name: (fd.get('first_name') || '').trim(),
      last_name: (fd.get('last_name') || '').trim(),
      address: (fd.get('address') || '').trim(),
      city: (fd.get('city') || '').trim(),
      postal_code: (fd.get('postal_code') || '').trim(),
      notes: (fd.get('notes') || '').trim(),
      payment_method: fd.get('payment_method') || 'cod',
      items: cart.map((i) => ({
        product_id: i.productId,
        // Preorder lines carry no size; send "" (server stores NULL) not "null".
        size: i.size == null ? '' : String(i.size),
        color: i.color,
        qty: i.qty,
      })),
    };

    this.lastEmail = payload.email;
    this.setBusy(true);
    try {
      const csrfToken = document.cookie.match(/(?:^|;\s*)_csrf=([^;]+)/)?.[1] ?? '';
      const res = await fetch('/api/orders', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        // 409 = a line is no longer in stock (server enforces inventory). Tell
        // the buyer to review their cart rather than showing a generic failure.
        this.errorEl.textContent = res.status === 409 ? this.errorStock : this.errorGeneric;
        this.errorEl.hidden = false;
        this.setBusy(false);
        return;
      }
      const data = await res.json();
      this.onSuccess(data);
    } catch {
      this.errorEl.textContent = this.errorGeneric;
      this.errorEl.hidden = false;
      this.setBusy(false);
    }
  }

  setBusy(busy) {
    this.submitBtn.disabled = busy;
    this.submitBtn.textContent = busy
      ? this.submitBtn.dataset.labelPlacing
      : this.submitBtn.dataset.labelPlace;
  }

  onSuccess(data) {
    // Funnel: reservation submitted (the order POST is already logged server-side;
    // this fires the Meta Lead event for ad optimisation/retargeting).
    track('Lead', { order_id: data.id, value: data.total_mkd, currency: 'MKD' });

    // Clear the cart and let the nav badge + drawer reset to empty.
    localStorage.removeItem(CART_KEY);
    window.dispatchEvent(new Event('cart:updated'));

    this.confirmOrderEl.textContent = '#' + data.id;
    // Email is optional now — only show the "we emailed you" note if one was given.
    if (this.confirmEmailNoteEl && this.lastEmail) {
      this.confirmEmailNoteEl.innerHTML = `${esc(this.labelEmailSent)} <strong>${esc(this.lastEmail)}</strong>. ${this.labelSpamNote}`;
      this.confirmEmailNoteEl.hidden = false;
    } else if (this.confirmEmailNoteEl) {
      this.confirmEmailNoteEl.hidden = true;
    }
    const bank = data.payment_method === 'bank_transfer';
    this.confirmBankEl.hidden = !bank;
    this.confirmCodEl.hidden = bank;

    this.gridEl.hidden = true;
    this.emptyEl.hidden = true;
    this.confirmEl.hidden = false;
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }
}

customElements.define('checkout-form', CheckoutForm);

export function initCheckoutForm() {
  // WebComponent automatically initializes on connection
}
