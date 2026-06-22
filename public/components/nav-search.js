// Nav search — opens a dropdown and filters the live product list client-side.
// There are only a handful of published products and /api/products already
// returns name/brand/slug/image/price, so we fetch it once (lazily, on first
// open) and filter in the browser rather than adding a backend search route.
// Product names and brand names are EN-only, so matching is against EN text.

let MKD_TO_EUR = 61;

function floorDenar(n) {
  // round down to nearest 10 — mirrors site.FloorDenar / cart-drawer.
  return Math.floor(n / 10) * 10;
}
function fmtMKD(n) {
  return String(floorDenar(n)).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}
function fmtEUR(n) {
  return String(Math.round(floorDenar(n) / MKD_TO_EUR));
}
function esc(s) {
  const d = document.createElement('div');
  d.textContent = s == null ? '' : String(s);
  return d.innerHTML;
}

// /a/x.webp → /a/x-card.webp (the 500px variant). Falls back to the original
// via the <img> onerror if the variant doesn't exist on disk.
function cardImg(url) {
  if (!url || url.startsWith('http')) return url;
  const dot = url.lastIndexOf('.');
  if (dot < 0) return url;
  return `${url.slice(0, dot)}-card${url.slice(dot)}`;
}

function getEls() {
  return {
    dropdown: document.getElementById('search-dropdown'),
    btn: document.getElementById('search-btn'),
    input: document.getElementById('search-input'),
    results: document.getElementById('search-results'),
  };
}

export function openSearch() {
  const { dropdown, btn, input } = getEls();
  if (!dropdown) return;
  dropdown.setAttribute('aria-hidden', 'false');
  dropdown.removeAttribute('inert'); // restore focus/interaction (attribute, not the
  // .inert property, which some in-app webviews don't reflect — see cart-drawer.js)
  btn.setAttribute('aria-expanded', 'true');
  input?.focus();
}

export function closeSearch() {
  const { dropdown, btn } = getEls();
  if (!dropdown || dropdown.getAttribute('aria-hidden') === 'true') return;
  dropdown.setAttribute('aria-hidden', 'true');
  dropdown.setAttribute('inert', ''); // take descendants out of the tab order while hidden
  btn.setAttribute('aria-expanded', 'false');
  btn?.focus();
}

// Lazily fetch the product list once; cache the promise so concurrent calls and
// every later keystroke reuse the same request.
let productsPromise = null;
function loadProducts() {
  if (!productsPromise) {
    productsPromise = fetch('/api/products', { headers: { 'X-Requested-With': 'fetch' } })
      .then((res) => (res.ok ? res.json() : []))
      .catch(() => {
        productsPromise = null; // allow a retry on the next keystroke
        return [];
      });
  }
  return productsPromise;
}

function renderMessage(results, text) {
  results.innerHTML = `<p class="search-dropdown__msg">${esc(text)}</p>`;
}

function renderResults(results, items, locale, showEur, currency) {
  if (!items.length) {
    renderMessage(results, results.dataset.msgEmpty);
    return;
  }
  results.innerHTML = items
    .map((p) => {
      const href = `/${locale}/products/${p.brand_slug}/${p.slug}`;
      const img = cardImg(p.image_url || '');
      const thumb = img
        ? `<img class="search-result__img" src="${esc(img)}" alt="" loading="lazy"
             onerror="this.onerror=null;this.src='${esc(p.image_url || '')}'">`
        : '<span class="search-result__img search-result__img--empty" aria-hidden="true"></span>';
      const eur = showEur
        ? `<span class="search-result__price-eur">${fmtEUR(p.price_mkd)} €</span>`
        : '';
      return `<a class="search-result" role="option" href="${esc(href)}">
        ${thumb}
        <span class="search-result__body">
          <span class="search-result__brand">${esc(p.brand_name)}</span>
          <span class="search-result__name">${esc(p.name)}</span>
        </span>
        <span class="search-result__price">
          <span class="search-result__price-mkd">${fmtMKD(p.price_mkd)} ${esc(currency)}</span>
          ${eur}
        </span>
      </a>`;
    })
    .join('');
}

async function runSearch(query) {
  const { results } = getEls();
  if (!results) return;

  const locale = document.documentElement.lang || 'mk';
  const showEur = results.dataset.showEur === '1';
  const currency = results.dataset.currency || 'MKD';
  MKD_TO_EUR = parseFloat(results.dataset.eurRate) || MKD_TO_EUR;

  const q = query.trim().toLowerCase();
  if (q.length < 2) {
    renderMessage(results, results.dataset.msgHint);
    return;
  }

  renderMessage(results, results.dataset.msgLoading);
  const products = await loadProducts();

  // Guard against a stale render: the input may have changed while awaiting.
  const { input } = getEls();
  if (input && input.value.trim().toLowerCase() !== q) return;

  const matches = products
    .filter((p) => {
      const hay = `${p.name} ${p.brand_name}`.toLowerCase();
      return hay.includes(q);
    })
    .slice(0, 8);

  renderResults(results, matches, locale, showEur, currency);
}

export function initNavSearch() {
  const { input, results } = getEls();

  document.addEventListener('click', (e) => {
    if (e.target.closest('#search-btn')) {
      openSearch();
    } else if (e.target.closest('#search-close')) {
      closeSearch();
    } else if (!e.target.closest('#search-dropdown')) {
      closeSearch();
    }
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeSearch();
  });

  if (!input || !results) return;

  // Seed the hint message so the dropdown isn't blank on first open.
  renderMessage(results, results.dataset.msgHint);

  let debounce;
  input.addEventListener('input', () => {
    clearTimeout(debounce);
    const value = input.value;
    debounce = setTimeout(() => runSearch(value), 150);
  });
}
