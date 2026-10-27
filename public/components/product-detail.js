import { track } from '../funnel.js';

export function initProductDetail() {
  // ── Stock data ────────────────────────────────────────
  // Build {color: {size: qty}} map from the embedded JSON.
  const stockEl = document.getElementById('stock-data');
  if (!stockEl) return;

  // Funnel: a product page was viewed (pixel-only; first-party already has the
  // GET in the access log). value is the product price in MKD denars (a plain
  // number, from data-price), currency MKD — consistent with the other events.
  const addBtn = document.getElementById('add-to-cart');
  track('ViewContent', {
    product_id: Number(addBtn?.dataset.productId) || 0,
    value: Number(addBtn?.dataset.price) || 0,
    currency: 'MKD',
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

  // Pull the authoritative live stock map, replacing whatever we have (SSR cache
  // or a pre-reconnect snapshot). Runs on load AND on every (re)connect so a stock
  // change that happened while the SSE stream was dropped can't leave us stale.
  function syncLiveStock() {
    return fetch(`/api/products/${productId}/stock`)
      .then((res) => {
        if (!res.ok) throw new Error('Failed to fetch live stock');
        return res.json();
      })
      .then((liveStock) => {
        for (const c in stockMap) delete stockMap[c];
        liveStock.forEach(({ eu_size, color, qty }) => {
          if (!stockMap[color]) stockMap[color] = {};
          stockMap[color][eu_size] = qty;
        });
        updateSizes();
      })
      .catch((err) => console.error('Error fetching live stock:', err));
  }

  // Fetch the live stock data immediately to bypass the 60-second SSR page cache.
  if (productId) {
    syncLiveStock();

    // Open SSE connection for real-time push updates
    const streamUrl = `/api/products/${productId}/stock/stream`;
    const eventSource = new EventSource(streamUrl);

    // On (re)connect, resync — closes the gap where an update fired while the
    // stream was down. Skips the very first connect (the load fetch above covers it).
    let sseConnected = false;
    eventSource.addEventListener('open', () => {
      if (sseConnected) syncLiveStock();
      sseConnected = true;
    });

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

    // A backgrounded tab (esp. mobile) suspends EventSource, so a size can sell
    // out while we're away. Resync whenever the tab becomes visible again — this
    // catches the common "opened, switched apps, came back to buy" case.
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') syncLiveStock();
    });

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
  const labelPreorder = realBtn?.dataset.labelPreorder || 'Reserve';
  const labelOrder = realBtn?.dataset.labelOrder || 'Order';
  // The model's mode (data/{brand}/catalog.json) decides what happens to a size
  // that is NOT in stock — per size, so a model can mix both:
  //   stock       → greyed, "Get notified" (not orderable)
  //   reservation → orderable, "Reserve", ~2–3 weeks note
  //   dropship    → orderable, "Order", ~7 days note
  // In-stock sizes are always "Add to cart" and decrement on order.
  const fulfilment = realBtn?.dataset.fulfilment || 'stock';
  const backorderable = fulfilment === 'reservation' || fulfilment === 'dropship';
  const notifyNote = document.getElementById('notify-note');
  const preorderNote = document.getElementById('preorder-note');
  const reserveHint = document.getElementById('reserve-hint');

  // qty>0 for the given size in the currently selected colour.
  function sizeInStock(size) {
    return ((stockMap[selectedColor] || {})[size] || 0) > 0;
  }

  // Any size of any colour on the shelf? (Decides the delivery note before a
  // size is picked: an all-backorder model shows it straight away.)
  function anyInStock() {
    return Object.values(stockMap).some((bySize) => Object.values(bySize).some((q) => q > 0));
  }

  // Drive both buttons' label + the helper notes from the current selection:
  //  • no size yet                      → "Select size".
  //  • size in stock                    → "Add to cart".
  //  • not in stock, reservation model  → "Reserve" + ~2–3 weeks note.
  //  • not in stock, dropship model     → "Order" + ~7 days note.
  //  • not in stock, stock model        → "Get notified about this size".
  function refreshBuyState() {
    let label = labelPick;
    let pick = true; // muted "not ready to buy" styling
    let showNotify = false;
    let showDelivery = backorderable && selectedSize == null && !anyInStock();

    if (selectedSize != null) {
      if (sizeInStock(selectedSize)) {
        label = labelAdd;
        pick = false;
      } else if (backorderable) {
        label = fulfilment === 'dropship' ? labelOrder : labelPreorder;
        pick = false;
        showDelivery = true;
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
    if (preorderNote) preorderNote.hidden = !showDelivery;
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
    // Same slug rule as shoeimport's colorFolder(): "White & Green" → "white-green".
    const key = color
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '');

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
      none: 'Not in stock — reserve this size, it arrives in 2–3 weeks.',
    },
    mk: {
      one: 'Останато е само уште 1 парче!',
      few: 'Останати се само уште неколку парчиња!',
      none: 'Нема на залиха — резервирај ја оваа големина, стига за 2–3 недели.',
    },
    sq: {
      one: 'Ka mbetur vetëm 1 copë!',
      few: 'Kanë mbetur vetëm edhe pak copë!',
      none: 'Nuk ka në stok — rezervo këtë masë, arrin për 2–3 javë.',
    },
  };

  // The fuzzy low-stock message for a size in the current colour, or null when
  // the size isn't low (>3 left). A reservation size that isn't on the shelf
  // gets the "not in stock — reserve" line instead. Shared by
  // the on-select warning and the on-hover preview so both read identically.
  function lowStockMessageFor(size) {
    if (size == null) return null;
    const byColor = stockMap[selectedColor] || {};
    const qty = byColor[size] || 0;
    if (
      qty <= 0 &&
      fulfilment === 'reservation' &&
      (stockMap[selectedColor] == null || size in byColor)
    ) {
      return lowStockMessages[locale]?.none || lowStockMessages.en.none;
    }
    if (qty === 1) return lowStockMessages[locale]?.one || lowStockMessages.en.one;
    if (qty === 2 || qty === 3) return lowStockMessages[locale]?.few || lowStockMessages.en.few;
    return null;
  }

  function setLowStockWarning(msg) {
    if (!lowStockWarning) return;
    // The line always occupies one row (CSS min-height), so setting empty text
    // clears the hint without shifting the buy section below it.
    lowStockWarning.textContent = msg || '';
  }

  function refreshLowStockWarning() {
    setLowStockWarning(lowStockMessageFor(selectedSize));
  }

  function updateSizes() {
    // Keep user's active size selected, or clear it if it is null (like when color changes).
    sizeBtns.forEach((b) => {
      const active = selectedSize != null && parseFloat(b.dataset.size) === selectedSize;
      b.classList.toggle('product__size-btn--active', active);
      b.setAttribute('aria-pressed', String(active));
    });

    // Size availability styling. Idempotent — safe to call repeatedly from
    // syncLiveStock()/SSE without accumulating state.
    //  • reservation/dropship model → no size is greyed: sizes not in stock are
    //    still orderable (ordered from the supplier).
    //  • stock model → out-of-stock sizes are greyed but stay clickable, so
    //    selecting one flips the button to "Get notified about this size".
    //  • a size the selected colour isn't made in (no stock row, e.g. a Be
    //    Lenka colour sold only in 36–41) → disabled in every mode.
    //  • reservation model → every size not on the shelf gets a "Reserve" tag
    //    (own colour) so it reads as orderable but slower. The hint line only
    //    shows when the colour mixes stock + reserve; an all-reservation colour
    //    already gets the delivery note under the buy button.
    const byColor = stockMap[selectedColor] || {};
    const knownColor = selectedColor != null && stockMap[selectedColor] != null;
    const isReservation = fulfilment === 'reservation';
    const mixed = isReservation && Object.values(byColor).some((q) => q > 0);
    let anyReserve = false;
    sizeBtns.forEach((btn) => {
      const size = parseFloat(btn.dataset.size);
      const offered = !knownColor || size in byColor;
      const empty = (byColor[size] || 0) <= 0;
      const oos = !offered || (!backorderable && empty);
      const reserve = isReservation && offered && empty;
      if (reserve && mixed) anyReserve = true;
      btn.classList.toggle('product__size-btn--oos', oos);
      btn.classList.toggle('product__size-btn--reserve', reserve);
      btn.disabled = !offered;
    });
    if (reserveHint) reserveHint.hidden = !anyReserve;

    refreshBuyState();
    refreshLowStockWarning();
  }

  sizeBtns.forEach((btn) => {
    btn.addEventListener('click', () => {
      if (btn.disabled) return;
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

    // Hover preview (desktop): show the hovered size's low-stock message; fall
    // back to the selected size's warning when the hovered size isn't low, and
    // restore the selected-size state on mouse-out.
    btn.addEventListener('mouseenter', () => {
      const msg = lowStockMessageFor(parseFloat(btn.dataset.size));
      setLowStockWarning(msg || lowStockMessageFor(selectedSize));
    });
    btn.addEventListener('mouseleave', refreshLowStockWarning);
  });

  // Pre-select the first colour on load (single or multi colour).
  if (colorBtns.length) selectColor(colorBtns[0].dataset.color);
  else refreshBuyState();

  // ── Add to cart (shared by both buttons) ──────────────
  function addToCart() {
    // A size must be picked first — we have to know exactly which pair to ship
    // (or to order from the supplier).
    if (selectedSize == null) {
      const grid = document.getElementById('size-select');
      if (grid) {
        grid.classList.add('product__sizes--error');
        grid.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      return;
    }

    // Not in stock on a "stock" model = "notify me", not orderable. Surface the
    // note and bail (no cart mutation). Reservation/dropship take the order.
    const backorder = !sizeInStock(selectedSize);
    if (backorder && !backorderable) {
      if (notifyNote) {
        notifyNote.hidden = false;
        notifyNote.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      return;
    }

    const d = realBtn.dataset;
    const cart = JSON.parse(localStorage.getItem('bosfoot_cart') || '[]');
    const size = selectedSize;
    // Backorder lines keep their own key namespace so they never merge with an
    // in-stock line for the same size/colour.
    const key = backorder
      ? `${d.productId}-preorder-${size}-${selectedColor}`
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
        // preorder = not from our shelf (ordered from the supplier); fulfilment
        // says which, for the cart label ("Reserved" vs "~7 days").
        preorder: backorder,
        fulfilment: backorder ? fulfilment : 'stock',
      });
    }
    localStorage.setItem('bosfoot_cart', JSON.stringify(cart));
    window.dispatchEvent(new Event('cart:updated')); // refresh nav badge + cart drawer
    window.dispatchEvent(new Event('cart:open')); // slide the drawer in with the new item

    // Funnel: the only step with no server request, so this beacon is what makes
    // add-to-cart visible (plus the Meta AddToCart event when the pixel is on).
    track('AddToCart', { product_id: parseInt(d.productId, 10), size, color: selectedColor });

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
