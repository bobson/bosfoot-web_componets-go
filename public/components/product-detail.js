export function initProductDetail() {
  // ── Stock data ────────────────────────────────────────
  // Build {color: {size: qty}} map from the embedded JSON.
  const stockEl = document.getElementById('stock-data');
  if (!stockEl) return;
  const rawStock = JSON.parse(stockEl.textContent || '[]');
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

  // ── Gallery carousel + colour filtering ──────────────
  // Native scroll-snap handles swipe. JS tracks the active slide via
  // IntersectionObserver and navigates with scrollIntoView. When a colour is
  // selected, slides whose image URL does not contain that colour's folder are
  // hidden (display:none), so the carousel only shows the chosen colour.
  const track = document.getElementById('gallery-track');
  const slides = track ? Array.from(track.querySelectorAll('.product__slide')) : [];
  const thumbs = Array.from(document.querySelectorAll('.product__thumb'));
  const prevBtn = document.getElementById('gallery-prev');
  const nextBtn = document.getElementById('gallery-next');
  const counter = document.getElementById('gallery-counter');

  // Extract the colour folder name from an image path and normalise spaces to
  // hyphens: /images/freet/feldom-3/images/olive-green/feldom-3-1.webp → 'olive-green'
  function colorFolder(src) {
    if (!src) return null;
    const parts = src.split('/');
    const idx = parts.lastIndexOf('images');
    return (idx !== -1 && idx + 2 < parts.length)
      ? parts[idx + 1].toLowerCase().replace(/\s+/g, '-')
      : null;
  }

  // Returns only the slides currently visible (not hidden by colour filter).
  function visibleSlides() {
    return slides.filter(s => s.style.display !== 'none');
  }

  let current = 0;

  const setCurrent = (i) => {
    current = i;
    const vis = visibleSlides();
    const visIdx = vis.indexOf(slides[i]);
    thumbs.forEach((t, k) => {
      const on = k === i;
      t.classList.toggle('product__thumb--active', on);
      t.setAttribute('aria-current', on ? 'true' : 'false');
    });
    if (counter) counter.textContent = `${visIdx + 1} / ${vis.length}`;
    if (prevBtn) prevBtn.disabled = visIdx <= 0;
    if (nextBtn) nextBtn.disabled = visIdx >= vis.length - 1;
  };

  // Navigate by visible position (prev = -1, next = +1, or absolute index).
  const goToVisible = (visIdx) => {
    const vis = visibleSlides();
    const clamped = Math.max(0, Math.min(vis.length - 1, visIdx));
    vis[clamped]?.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
  };

  const goToSlide = (arrayIdx) => {
    slides[arrayIdx]?.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
  };

  if (slides.length > 1) {
    prevBtn?.addEventListener('click', () => {
      const visIdx = visibleSlides().indexOf(slides[current]);
      goToVisible(visIdx - 1);
    });
    nextBtn?.addEventListener('click', () => {
      const visIdx = visibleSlides().indexOf(slides[current]);
      goToVisible(visIdx + 1);
    });
    thumbs.forEach((t, i) => t.addEventListener('click', () => goToSlide(i)));

    const io = new IntersectionObserver((entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting && e.intersectionRatio >= 0.6) {
          const i = slides.indexOf(e.target);
          if (i !== -1) setCurrent(i);
        }
      });
    }, { root: track, threshold: 0.6 });
    slides.forEach((s) => io.observe(s));

    setCurrent(0);
  } else if (slides.length <= 1) {
    document.getElementById('gallery-controls')?.remove();
    document.getElementById('gallery-thumbs')?.remove();
  }

  // ── Colour selection ──────────────────────────────────
  const colorBtns = Array.from(document.querySelectorAll('.product__color-btn'));
  const colorNameEl = document.getElementById('color-name');
  const multiColor = colorBtns.length > 1;

  function applyColorFilter(color) {
    if (!multiColor) return; // single-colour: nothing to filter
    const key = color.toLowerCase().replace(/\s+/g, '-');
    let firstIdx = -1;
    slides.forEach((slide, i) => {
      const src = slide.querySelector('img')?.getAttribute('src') || '';
      const folder = colorFolder(src);
      const show = !folder || folder === key;
      slide.style.display = show ? '' : 'none';
      if (thumbs[i]) thumbs[i].style.display = show ? '' : 'none';
      if (show && firstIdx === -1) firstIdx = i;
    });
    if (firstIdx !== -1) {
      goToSlide(firstIdx);
      setCurrent(firstIdx);
    }
  }

  function selectColor(color) {
    selectedColor = color;
    selectedSize  = null;

    colorBtns.forEach(b => {
      const active = b.dataset.color === color;
      b.classList.toggle('product__color-btn--active', active);
      b.setAttribute('aria-pressed', String(active));
    });

    if (colorNameEl) colorNameEl.textContent = color;
    applyColorFilter(color);
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

    // Reservation mode (pre-launch): every size is reservable regardless of
    // stock, so we never disable a size here. Restore the per-size OOS check
    // (using stockMap[selectedColor]) when switching back to real selling.
    sizeBtns.forEach(btn => {
      btn.disabled = false;
      btn.classList.remove('product__size-btn--oos');
      btn.setAttribute('aria-disabled', 'false');
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
    window.dispatchEvent(new Event('cart:open'));     // slide the drawer in with the new item

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
}
