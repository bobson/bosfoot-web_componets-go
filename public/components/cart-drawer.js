// <cart-drawer> — an HTML Web Component (custom element, light DOM, no shadow).
// The static chrome is server-rendered inside the element; this class only adds
// behaviour and renders the line items from localStorage. Global CSS (cart.css)
// styles it normally because there's no shadow boundary.

const CART_KEY = 'bosfoot_cart';
// Default rate; overridden at connect from the data-eur-rate attribute, which
// the server injects from internal/site (the single source of truth).
let MKD_TO_EUR = 61;

function floorDenar(n) {
  // round down to the nearest 10 so prices end in 0 — mirrors site.FloorDenar.
  return Math.floor(n / 10) * 10;
}
function fmtMKD(n) {
  // space thousands separator, mirrors formatMKD on the server: 6200 -> "6 200"
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

class CartDrawer extends HTMLElement {
  connectedCallback() {
    this.currency = this.dataset.currency || 'MKD';
    this.showEur = this.dataset.showEur === '1';
    MKD_TO_EUR = parseFloat(this.dataset.eurRate) || MKD_TO_EUR;
    this.labels = {
      size: this.dataset.labelSize || 'Size',
      remove: this.dataset.labelRemove || 'Remove',
    };

    this.overlay = this.querySelector('[data-cart-overlay]');
    this.panel = this.querySelector('.cart__panel');
    this.itemsEl = this.querySelector('[data-cart-items]');
    this.emptyEl = this.querySelector('[data-cart-empty]');
    this.footerEl = this.querySelector('[data-cart-footer]');
    this.subtotalEl = this.querySelector('[data-cart-subtotal]');

    // Open from the nav cart button (lives outside this element).
    document.getElementById('cart-btn')?.addEventListener('click', () => this.open());

    // Close: × button, "continue", empty-CTA, overlay, Escape.
    this.querySelectorAll('[data-cart-close]').forEach((el) =>
      el.addEventListener('click', () => this.close()),
    );
    this.overlay?.addEventListener('click', () => this.close());
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && this.classList.contains('cart--open')) this.close();
    });

    // Quantity steppers + remove, via event delegation.
    this.itemsEl.addEventListener('click', (e) => this.onItemClick(e));

    // Open on demand — dispatched by the product page right after an item is
    // added, so the drawer slides in showing what was just added.
    window.addEventListener('cart:open', () => this.open());

    // Keep in sync with changes from the product page (same tab) and other tabs.
    window.addEventListener('cart:updated', () => this.render());
    window.addEventListener('storage', (e) => {
      if (e.key === CART_KEY) this.render();
    });

    this.render();
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

  write(cart) {
    localStorage.setItem(CART_KEY, JSON.stringify(cart));
    window.dispatchEvent(new Event('cart:updated')); // updates the nav badge too
  }

  open() {
    this.classList.add('cart--open');
    this.panel.setAttribute('aria-hidden', 'false');
    this.overlay.setAttribute('aria-hidden', 'false');
    document.body.classList.add('cart-open');
  }

  close() {
    this.classList.remove('cart--open');
    this.panel.setAttribute('aria-hidden', 'true');
    this.overlay.setAttribute('aria-hidden', 'true');
    document.body.classList.remove('cart-open');
  }

  onItemClick(e) {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;
    const key = btn.closest('[data-key]')?.dataset.key;
    if (!key) return;

    const cart = this.read();
    const item = cart.find((i) => i.key === key);
    if (!item) return;

    if (btn.dataset.action === 'inc') item.qty++;
    else if (btn.dataset.action === 'dec') item.qty--;
    else if (btn.dataset.action === 'remove') item.qty = 0;

    this.write(cart.filter((i) => i.qty > 0));
    this.render();
  }

  price(n) {
    let s = `${fmtMKD(n)} ${this.currency}`;
    if (this.showEur) s += ` · ${fmtEUR(n)} €`;
    return s;
  }

  render() {
    const cart = this.read();

    if (!cart.length) {
      this.itemsEl.innerHTML = '';
      this.emptyEl.hidden = false;
      this.footerEl.hidden = true;
      return;
    }

    this.emptyEl.hidden = true;
    this.footerEl.hidden = false;
    this.itemsEl.innerHTML = cart.map((i) => this.row(i)).join('');

    const subtotal = cart.reduce((s, i) => s + i.price * i.qty, 0);
    this.subtotalEl.textContent = this.price(subtotal);
  }

  row(i) {
    const img = i.imageUrl
      ? `<img class="cart-item__img" src="${esc(i.imageUrl)}" alt="${esc(i.productName)}" loading="lazy">`
      : `<div class="cart-item__img"></div>`;
    return `
      <article class="cart-item" data-key="${esc(i.key)}">
        ${img}
        <div class="cart-item__info">
          <p class="cart-item__brand text-xs text-muted">${esc(i.brandName)}</p>
          <p class="cart-item__name font-medium">${esc(i.productName)}</p>
          <p class="cart-item__variant text-xs text-muted">${esc(this.labels.size)} ${esc(i.size)} · ${esc(i.color)}</p>
          <div class="cart-item__bottom">
            <div class="cart-item__qty">
              <button type="button" class="cart-item__step" data-action="dec" aria-label="−">−</button>
              <span class="cart-item__qty-val">${esc(i.qty)}</span>
              <button type="button" class="cart-item__step" data-action="inc" aria-label="+">+</button>
            </div>
            <span class="cart-item__price font-semibold">${esc(this.price(i.price * i.qty))}</span>
          </div>
        </div>
        <button type="button" class="cart-item__remove" data-action="remove" aria-label="${esc(this.labels.remove)}">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
        </button>
      </article>`;
  }
}

customElements.define('cart-drawer', CartDrawer);

export function initCartDrawer() {
  // WebComponent automatically initializes on connection
}
