export function initListingFilter() {
  const sidebar = document.getElementById('filter-sidebar');
  const overlay = document.getElementById('filter-overlay');
  const openBtn = document.getElementById('filter-open');
  const closeBtn = document.getElementById('filter-close');
  const applyBtn = document.getElementById('filter-apply');
  const resetBtn = document.getElementById('filter-reset');
  const badge = document.getElementById('filter-badge');
  const countEl = document.getElementById('filter-count');
  const noRes = document.getElementById('filter-noresults');
  const cards = Array.from(document.querySelectorAll('.product-card'));

  if (!sidebar) return; // Guard for non-listing pages

  // pending = selections visible in the sidebar (not yet applied to the grid)
  // applied = what is actually filtering the grid
  const dims = ['brand', 'gender', 'activity', 'size', 'color'];
  const pending = Object.fromEntries(dims.map((d) => [d, new Set()]));
  const applied = Object.fromEntries(dims.map((d) => [d, new Set()]));

  // ── Drawer ────────────────────────────────────────────

  function openDrawer() {
    sidebar.classList.add('filter-sidebar--open');
    overlay.classList.add('filter-overlay--visible');
    document.body.classList.add('filter-open');
    openBtn?.setAttribute('aria-expanded', 'true');
  }

  function closeDrawer() {
    sidebar.classList.remove('filter-sidebar--open');
    overlay.classList.remove('filter-overlay--visible');
    document.body.classList.remove('filter-open');
    openBtn?.setAttribute('aria-expanded', 'false');
  }

  openBtn?.addEventListener('click', openDrawer);
  closeBtn?.addEventListener('click', closeDrawer);
  overlay?.addEventListener('click', closeDrawer);

  // ── Chip selection (updates pending only) ─────────────

  document.querySelectorAll('.filter-chip').forEach((chip) => {
    chip.addEventListener('click', () => {
      const dim = chip.dataset.filter;
      const val = chip.dataset.value;
      if (pending[dim].has(val)) {
        pending[dim].delete(val);
        chip.classList.remove('filter-chip--active');
        chip.setAttribute('aria-pressed', 'false');
      } else {
        pending[dim].add(val);
        chip.classList.add('filter-chip--active');
        chip.setAttribute('aria-pressed', 'true');
      }
    });
  });

  // ── Apply: commit pending → applied, update grid ──────

  applyBtn?.addEventListener('click', () => {
    dims.forEach((d) => {
      applied[d] = new Set(pending[d]);
    });
    runFilter();
    closeDrawer();
  });

  // ── Reset: clear everything, show all products ────────

  resetBtn?.addEventListener('click', () => {
    dims.forEach((d) => {
      pending[d].clear();
      applied[d].clear();
    });
    document.querySelectorAll('.filter-chip--active').forEach((c) => {
      c.classList.remove('filter-chip--active');
      c.setAttribute('aria-pressed', 'false');
    });
    runFilter();
    closeDrawer();
  });

  // ── Filter logic ──────────────────────────────────────

  function runFilter() {
    let visible = 0;
    cards.forEach((card) => {
      const show = matches(card);
      card.hidden = !show;
      if (show) visible++;
    });

    if (countEl) countEl.textContent = visible;
    if (noRes) noRes.hidden = visible > 0;

    const total = dims.reduce((n, d) => n + applied[d].size, 0);
    if (badge) {
      badge.textContent = total;
      badge.hidden = total === 0;
    }

    syncURL();
  }

  function matches(card) {
    const d = card.dataset;
    if (applied.brand.size && !applied.brand.has(d.brand || '')) return false;
    if (applied.gender.size && !applied.gender.has(d.gender || '')) return false;
    if (applied.activity.size) {
      const v = (d.activities || '').split('|').filter(Boolean);
      if (![...applied.activity].some((a) => v.includes(a))) return false;
    }
    if (applied.size.size) {
      const v = (d.sizes || '').split('|').filter(Boolean);
      if (![...applied.size].some((s) => v.includes(s))) return false;
    }
    if (applied.color.size) {
      const v = (d.colors || '').split('|').filter(Boolean);
      if (![...applied.color].some((c) => v.includes(c))) return false;
    }
    return true;
  }

  function syncURL() {
    const params = new URLSearchParams();
    dims.forEach((d) => {
      if (applied[d].size) params.set(d, [...applied[d]].join(','));
    });
    const qs = params.toString();
    history.replaceState(null, '', qs ? `${location.pathname}?${qs}` : location.pathname);
  }

  // ── Restore from URL on load ──────────────────────────

  const urlParams = new URLSearchParams(location.search);
  let restored = false;
  dims.forEach((dim) => {
    const v = urlParams.get(dim);
    if (!v) return;
    v.split(',').forEach((val) => {
      pending[dim].add(val);
      applied[dim].add(val);
      const chip = document.querySelector(
        `.filter-chip[data-filter="${dim}"][data-value="${val}"]`,
      );
      if (chip) {
        chip.classList.add('filter-chip--active');
        chip.setAttribute('aria-pressed', 'true');
      }
    });
    restored = true;
  });
  if (restored) runFilter();
}
