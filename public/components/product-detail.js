import { track } from '../analytics.js';

export function initProductDetail() {
  // ── Stock data ────────────────────────────────────────
  // Build {color: {size: qty}} map from the embedded JSON.
  const stockEl = document.getElementById('stock-data');
  if (!stockEl) return;

  // Funnel: a product page was viewed (pixel-only; first-party already has the
  // GET in the access log).
  track('ViewContent', {
    product_id: Number(document.getElementById('add-to-cart')?.dataset.productId) || 0,
  });
  const rawStock = JSON.parse(stockEl.textContent || '[]');
  const stockMap = {};
  rawStock.forEach(({ eu_size, color, qty }) => {
    if (!stockMap[color]) stockMap[color] = {};
    stockMap[color][eu_size] = qty;
  });

  let selectedColor = null;
  let selectedSize = null;

  const productId = Number(document.getElementById('add-to-cart')?.dataset.productId) || 0;

  // Fetch the live stock data immediately to bypass the 60-second SSR page cache.
  if (productId) {
    fetch(`/api/products/${productId}/stock`)
      .then((res) => {
        if (!res.ok) throw new Error('Failed to fetch live stock');
        return res.json();
      })
      .then((liveStock) => {
        // Clear and rebuild stockMap with live data
        for (const c in stockMap) delete stockMap[c];
        liveStock.forEach(({ eu_size, color, qty }) => {
          if (!stockMap[color]) stockMap[color] = {};
          stockMap[color][eu_size] = qty;
        });
        // Update the UI sizes
        updateSizes();
      })
      .catch((err) => console.error('Error fetching live stock:', err));

    // Open SSE connection for real-time push updates
    const streamUrl = `/api/products/${productId}/stock/stream`;
    const eventSource = new EventSource(streamUrl);

    eventSource.addEventListener('stock_update', (e) => {
      try {
        const update = JSON.parse(e.data);
        const { eu_size, color, qty } = update;

        if (!stockMap[color]) stockMap[color] = {};
        stockMap[color][eu_size] = qty;

        // Refresh UI
        updateSizes();
      } catch (err) {
        console.error('Error parsing SSE stock update:', err);
      }
    });

    eventSource.onerror = (err) => {
      console.error('SSE connection error:', err);
    };

    // Close SSE stream when navigating away
    window.addEventListener('beforeunload', () => {
      eventSource.close();
    });
  }

  // ── Buy buttons (in-page + floating) ──────────────────
  const realBtn = document.getElementById('add-to-cart');
  const floatBtn = document.getElementById('add-to-cart-floating');
  const buyBtns = [realBtn, floatBtn].filter(Boolean);
  const labelPick = realBtn?.dataset.labelPick || 'Select size';
  const labelAdd = realBtn?.dataset.labelAdd || 'Add to cart';
  const labelNotify = realBtn?.dataset.labelNotify || 'Get notified about this size';
  const labelPreorder = realBtn?.dataset.labelPreorder || 'Preorder now';
  // A product with NO stock at all is a "preorder" product: the size grid is
  // display-only and the button is a standing "Preorder now" (no size needed).
  const preorderMode = realBtn?.dataset.inStock !== '1';
  const notifyNote = document.getElementById('notify-note');
  const preorderNote = document.getElementById('preorder-note');

  // qty>0 for the given size in the currently selected colour.
  function sizeInStock(size) {
    return ((stockMap[selectedColor] || {})[size] || 0) > 0;
  }

  // Drive both buttons' label + the helper notes from the current selection:
  //  • preorder product        → "Preorder now" (always ready), preorder note.
  //  • stocked, no size yet     → "Select size".
  //  • stocked, in-stock size   → "Add to cart".
  //  • stocked, out-of-stock    → "Get notified about this size", notify note.
  function refreshBuyState() {
    let label = labelPick;
    let pick = true; // muted "not ready to buy" styling
    let showNotify = false;

    if (preorderMode) {
      label = labelPreorder;
      pick = false;
    } else if (selectedSize != null) {
      if (sizeInStock(selectedSize)) {
        label = labelAdd;
        pick = false;
      } else {
        label = labelNotify;
        showNotify = true;
      }
    }

    buyBtns.forEach((b) => {
      b.textContent = label;
      b.classList.toggle('product__add-to-cart--pick', pick);
    });
    if (notifyNote) notifyNote.hidden = !showNotify;
    if (preorderNote) preorderNote.hidden = !preorderMode;
  }

  // ── Gallery carousel + colour filtering ──────────────
  // Native scroll-snap handles swipe. JS tracks the active slide via
  // IntersectionObserver and navigates with scrollIntoView. When a colour is
  // selected, slides whose image URL does not contain that colour's folder are
  // hidden (display:none), so the carousel only shows the chosen colour.
  const galleryTrack = document.getElementById('gallery-track');
  const slides = galleryTrack ? Array.from(galleryTrack.querySelectorAll('.product__slide')) : [];
  const thumbs = Array.from(document.querySelectorAll('.product__thumb'));
  const prevBtn = document.getElementById('gallery-prev');
  const nextBtn = document.getElementById('gallery-next');
  const counter = document.getElementById('gallery-counter');

  // Extract the colour folder name from an image path and normalise spaces to
  // hyphens: /images/freet/feldom-3/images/olive-green/feldom-3-1.webp → 'olive-green'
  function colorFolder(src) {
    if (!src) return null;
    const path = src.split('?')[0]; // strip ?v=...
    const parts = path.split('/');
    const idx = parts.lastIndexOf('images');
    return idx !== -1 && idx + 2 < parts.length
      ? parts[idx + 1].toLowerCase().replace(/\s+/g, '-')
      : null;
  }

  // Returns only the slides currently visible (not hidden by colour filter).
  function visibleSlides() {
    return slides.filter((s) => !s.hasAttribute('hidden'));
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
    const handleNav = (dir) => {
      const vis = visibleSlides();
      const visIdx = vis.indexOf(slides[current]);
      goToVisible(visIdx + dir);
    };

    prevBtn?.addEventListener('click', () => handleNav(-1));
    nextBtn?.addEventListener('click', () => handleNav(1));
    thumbs.forEach((t, i) => t.addEventListener('click', () => goToSlide(i)));

    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((e) => {
          if (e.isIntersecting && e.intersectionRatio >= 0.6) {
            const i = slides.indexOf(e.target);
            if (i !== -1) setCurrent(i);
          }
        });
      },
      { root: galleryTrack, threshold: 0.6 },
    );
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

    // Guard: if no slide's path matches this colour (e.g. a data gap where the
    // colour is declared but its gallery rows were never seeded), filtering
    // would hide every slide and leave a blank carousel. Fall back to showing
    // all images so the gallery degrades to "shows something" instead of empty.
    const anyMatch = slides.some((slide) => {
      const img = slide.querySelector('img');
      const folder = colorFolder(img ? img.getAttribute('src') || '' : '');
      return folder === key;
    });

    let firstIdx = -1;
    slides.forEach((slide, i) => {
      const img = slide.querySelector('img');
      const src = img ? img.getAttribute('src') || '' : '';
      const folder = colorFolder(src);
      const show = !anyMatch || !folder || folder === key;

      if (show) {
        slide.removeAttribute('hidden');
        if (thumbs[i]) thumbs[i].removeAttribute('hidden');
        if (firstIdx === -1) firstIdx = i;
      } else {
        slide.setAttribute('hidden', '');
        if (thumbs[i]) thumbs[i].setAttribute('hidden', '');
      }
    });

    if (firstIdx !== -1) {
      // Force layout recalculation then scroll
      requestAnimationFrame(() => {
        goToSlide(firstIdx);
        setCurrent(firstIdx);
      });
    }
  }

  function selectColor(color) {
    if (!color) return;
    selectedColor = color;
    selectedSize = null;

    colorBtns.forEach((b) => {
      const active = b.dataset.color === color;
      b.classList.toggle('product__color-btn--active', active);
      b.setAttribute('aria-pressed', String(active));
    });

    if (colorNameEl) colorNameEl.textContent = color;
    applyColorFilter(color);
    updateSizes();

    // Update the cart button image to match the selected color
    const firstVisible = visibleSlides()[0];
    if (firstVisible) {
      const img = firstVisible.querySelector('img');
      if (img && realBtn) {
        realBtn.dataset.imageUrl = img.getAttribute('src');
      }
    }
  }

  colorBtns.forEach((btn) => {
    // Use both click and touchstart for better mobile response
    const handler = (e) => {
      e.preventDefault();
      selectColor(btn.dataset.color);
    };
    btn.addEventListener('click', handler);
  });

  // ── Size availability ─────────────────────────────────
  const sizeBtns = Array.from(document.querySelectorAll('.product__size-btn'));
  const lowStockWarning = document.getElementById('low-stock-warning');
  const locale = document.documentElement.lang || 'en';

  const lowStockMessages = {
    en: {
      one: 'Only 1 item left in stock!',
      few: 'Only a few items left in stock!',
    },
    mk: {
      one: 'Останато е само уште 1 парче!',
      few: 'Останати се само уште неколку парчиња!',
    },
    sq: {
      one: 'Ka mbetur vetëm 1 copë!',
      few: 'Kanë mbetur vetëm edhe pak copë!',
    },
  };

  function refreshLowStockWarning() {
    if (!lowStockWarning) return;
    if (selectedSize == null || preorderMode) {
      lowStockWarning.hidden = true;
      return;
    }

    const byColor = stockMap[selectedColor] || {};
    const qty = byColor[selectedSize] || 0;

    if (qty === 1) {
      lowStockWarning.textContent = lowStockMessages[locale]?.one || lowStockMessages.en.one;
      lowStockWarning.hidden = false;
    } else if (qty === 2 || qty === 3) {
      lowStockWarning.textContent = lowStockMessages[locale]?.few || lowStockMessages.en.few;
      lowStockWarning.hidden = false;
    } else {
      lowStockWarning.hidden = true;
    }
  }

  function updateSizes() {
    // Keep user's active size selected, or clear it if it is null (like when color changes).
    sizeBtns.forEach((b) => {
      const active = selectedSize != null && parseFloat(b.dataset.size) === selectedSize;
      b.classList.toggle('product__size-btn--active', active);
      b.setAttribute('aria-pressed', String(active));
    });

    // Grey the sizes that are out of stock for the selected colour.
    //  • preorder product → every size is greyed AND inert (display-only); the
    //    button is a standing "Preorder now" that needs no size.
    //  • stocked product  → out-of-stock sizes are greyed but stay clickable, so
    //    selecting one flips the button to "Get notified about this size".
    const byColor = stockMap[selectedColor] || {};
    sizeBtns.forEach((btn) => {
      const oos = (byColor[parseFloat(btn.dataset.size)] || 0) <= 0;
      btn.classList.toggle('product__size-btn--oos', oos);
      btn.disabled = preorderMode; // inert only for no-stock products
      btn.setAttribute('aria-disabled', String(preorderMode));
    });

    refreshBuyState();
    refreshLowStockWarning();
  }

  sizeBtns.forEach((btn) => {
    btn.addEventListener('click', () => {
      if (btn.disabled) return; // preorder products: sizes are display-only
      selectedSize = parseFloat(btn.dataset.size);
      sizeBtns.forEach((b) => {
        const active = parseFloat(b.dataset.size) === selectedSize;
        b.classList.toggle('product__size-btn--active', active);
        b.setAttribute('aria-pressed', String(active));
      });
      document.getElementById('size-select')?.classList.remove('product__sizes--error');
      refreshBuyState();
      refreshLowStockWarning();
    });
  });

  // Pre-select the first colour on load (single or multi colour).
  if (colorBtns.length) selectColor(colorBtns[0].dataset.color);
  else refreshBuyState();

  // ── Add to cart (shared by both buttons) ──────────────
  function addToCart() {
    // Preorder product: no size needed — add straight away as a preorder line.
    // Stocked product: a size must be picked first.
    if (!preorderMode && selectedSize == null) {
      const grid = document.getElementById('size-select');
      if (grid) {
        grid.classList.add('product__sizes--error');
        grid.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      return;
    }

    // Out-of-stock size on a stocked product = "notify me", not orderable.
    // Surface the note and bail (no cart mutation).
    if (!preorderMode && selectedSize != null && !sizeInStock(selectedSize)) {
      if (notifyNote) {
        notifyNote.hidden = false;
        notifyNote.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      return;
    }

    const d = realBtn.dataset;
    const cart = JSON.parse(localStorage.getItem('bosfoot_cart') || '[]');
    // Preorder lines carry no size; key on color alone so they merge sanely.
    const size = preorderMode ? null : selectedSize;
    const key = preorderMode
      ? `${d.productId}-preorder-${selectedColor}`
      : `${d.productId}-${size}-${selectedColor}`;
    const existing = cart.find((i) => i.key === key);
    if (existing) {
      existing.qty++;
    } else {
      cart.push({
        key,
        productId: parseInt(d.productId, 10),
        productName: d.productName,
        brandName: d.brandName,
        imageUrl: d.imageUrl,
        size,
        color: selectedColor,
        price: parseInt(d.price, 10),
        qty: 1,
        preorder: preorderMode,
      });
    }
    localStorage.setItem('bosfoot_cart', JSON.stringify(cart));
    window.dispatchEvent(new Event('cart:updated')); // refresh nav badge + cart drawer
    window.dispatchEvent(new Event('cart:open')); // slide the drawer in with the new item

    // Funnel: the only step with no server request, so this beacon is what makes
    // add-to-cart visible (plus the Meta AddToCart event when the pixel is on).
    track('AddToCart', { product_id: parseInt(d.productId, 10) });

    // Brief confirmation on both buttons, then restore the live state.
    buyBtns.forEach((b) => {
      b.textContent = '✓';
      b.classList.remove('product__add-to-cart--pick');
      b.classList.add('product__add-to-cart--added');
    });
    setTimeout(() => {
      buyBtns.forEach((b) => b.classList.remove('product__add-to-cart--added'));
      refreshBuyState();
    }, 1600);
  }

  buyBtns.forEach((b) => b.addEventListener('click', addToCart));

  // ── Floating bar visibility ───────────────────────────
  // Show the floating bar only while the in-page button is off-screen.
  const floatBar = document.getElementById('floating-buy');
  if (floatBar && realBtn && 'IntersectionObserver' in window) {
    const obs = new IntersectionObserver(
      ([entry]) => {
        floatBar.classList.toggle('product__floating--visible', !entry.isIntersecting);
        floatBar.setAttribute('aria-hidden', String(entry.isIntersecting));
      },
      { threshold: 0 },
    );
    obs.observe(realBtn);
  }
}
