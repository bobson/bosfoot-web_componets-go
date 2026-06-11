(function () {
  'use strict';

  // ── Stock data ────────────────────────────────────────
  // Build {color: {size: qty}} map from the embedded JSON.
  const rawStock = JSON.parse(document.getElementById('stock-data')?.textContent || '[]');
  const stockMap = {};
  rawStock.forEach(({ eu_size, color, qty }) => {
    if (!stockMap[color]) stockMap[color] = {};
    stockMap[color][eu_size] = qty;
  });

  let selectedColor = null;
  let selectedSize  = null;

  // ── Buy buttons (in-page + floating) ──────────────────
  const realBtn = document.getElementById('add-to-cart');
  const floatBtn = document.getElementById('add-to-cart-floating');
  const buyBtns = [realBtn, floatBtn].filter(Boolean);
  const labelPick = realBtn?.dataset.labelPick || 'Select size';
  const labelAdd  = realBtn?.dataset.labelAdd  || 'Add to cart';

  // Reflect selection state on both buttons: "Select size" until a size is
  // chosen (colour is always pre-selected), then "Add to cart".
  function refreshBuyState() {
    const ready = selectedSize != null;
    buyBtns.forEach(b => {
      b.textContent = ready ? labelAdd : labelPick;
      b.classList.toggle('product__add-to-cart--pick', !ready);
    });
  }

  // ── Gallery ───────────────────────────────────────────
  const mainImg = document.getElementById('gallery-main');

  document.querySelectorAll('.product__thumb').forEach((btn, i) => {
    if (i === 0) btn.classList.add('product__thumb--active');
    btn.addEventListener('click', () => {
      if (mainImg) {
        mainImg.style.opacity = '0';
        setTimeout(() => { mainImg.src = btn.dataset.src; mainImg.style.opacity = ''; }, 150);
      }
      document.querySelectorAll('.product__thumb').forEach(b => b.classList.remove('product__thumb--active'));
      btn.classList.add('product__thumb--active');
    });
  });

  // ── Colour selection ──────────────────────────────────
  const colorBtns = Array.from(document.querySelectorAll('.product__color-btn'));
  const colorNameEl = document.getElementById('color-name');

  function selectColor(color) {
    selectedColor = color;
    selectedSize  = null; // changing colour invalidates the chosen size

    colorBtns.forEach(b => {
      const active = b.dataset.color === color;
      b.classList.toggle('product__color-btn--active', active);
      b.setAttribute('aria-pressed', String(active));
    });

    if (colorNameEl) colorNameEl.textContent = color;
    updateSizes();
  }

  colorBtns.forEach(btn => btn.addEventListener('click', () => selectColor(btn.dataset.color)));

  // ── Size availability ─────────────────────────────────
  const sizeBtns = Array.from(document.querySelectorAll('.product__size-btn'));

  function updateSizes() {
    // Deselect current size when colour changes.
    sizeBtns.forEach(b => {
      b.classList.remove('product__size-btn--active');
      b.setAttribute('aria-pressed', 'false');
    });

    const sizes = selectedColor ? (stockMap[selectedColor] || {}) : {};
    sizeBtns.forEach(btn => {
      const qty = sizes[parseFloat(btn.dataset.size)] ?? 0;
      const oos = qty === 0;
      btn.disabled = oos;
      btn.classList.toggle('product__size-btn--oos', oos);
      btn.setAttribute('aria-disabled', String(oos));
    });

    refreshBuyState();
  }

  sizeBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      if (btn.disabled) return;
      selectedSize = parseFloat(btn.dataset.size);
      sizeBtns.forEach(b => {
        const active = parseFloat(b.dataset.size) === selectedSize;
        b.classList.toggle('product__size-btn--active', active);
        b.setAttribute('aria-pressed', String(active));
      });
      document.getElementById('size-select')?.classList.remove('product__sizes--error');
      refreshBuyState();
    });
  });

  // Pre-select the first colour on load (single or multi colour).
  if (colorBtns.length) selectColor(colorBtns[0].dataset.color);
  else refreshBuyState();

  // ── Add to cart (shared by both buttons) ──────────────
  function addToCart() {
    if (selectedSize == null) {
      // Not ready: surface the size selector and flag it.
      const grid = document.getElementById('size-select');
      if (grid) {
        grid.classList.add('product__sizes--error');
        grid.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      return;
    }

    const d = realBtn.dataset;
    const cart = JSON.parse(localStorage.getItem('bosfoot_cart') || '[]');
    const key  = `${d.productId}-${selectedSize}-${selectedColor}`;
    const existing = cart.find(i => i.key === key);
    if (existing) {
      existing.qty++;
    } else {
      cart.push({
        key,
        productId:   parseInt(d.productId, 10),
        productName: d.productName,
        brandName:   d.brandName,
        imageUrl:    d.imageUrl,
        size:        selectedSize,
        color:       selectedColor,
        price:       parseInt(d.price, 10),
        qty:         1,
      });
    }
    localStorage.setItem('bosfoot_cart', JSON.stringify(cart));
    window.dispatchEvent(new Event('cart:updated')); // refresh nav badge + cart drawer

    // Brief confirmation on both buttons, then restore the live state.
    buyBtns.forEach(b => {
      b.textContent = '✓';
      b.classList.remove('product__add-to-cart--pick');
      b.classList.add('product__add-to-cart--added');
    });
    setTimeout(() => {
      buyBtns.forEach(b => b.classList.remove('product__add-to-cart--added'));
      refreshBuyState();
    }, 1600);
  }

  buyBtns.forEach(b => b.addEventListener('click', addToCart));

  // ── Floating bar visibility ───────────────────────────
  // Show the floating bar only while the in-page button is off-screen.
  const floatBar = document.getElementById('floating-buy');
  if (floatBar && realBtn && 'IntersectionObserver' in window) {
    const obs = new IntersectionObserver(([entry]) => {
      floatBar.classList.toggle('product__floating--visible', !entry.isIntersecting);
      floatBar.setAttribute('aria-hidden', String(entry.isIntersecting));
    }, { threshold: 0 });
    obs.observe(realBtn);
  }
}());
