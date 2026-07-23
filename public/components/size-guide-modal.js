export function initSizeGuideModal() {
  const openBtn = document.getElementById('open-size-guide');
  const modal = document.getElementById('size-guide-modal');
  const closeBtn = modal?.querySelector('.sg-modal__close');

  if (!modal || !openBtn) return;

  // Open modal
  openBtn.addEventListener('click', () => {
    modal.showModal();
    document.body.style.overflow = 'hidden';
  });

  // Close modal
  const closeModal = () => {
    modal.close();
    document.body.style.overflow = '';
  };

  closeBtn?.addEventListener('click', closeModal);

  // Close on backdrop click (click outside the dialog)
  modal.addEventListener('click', (e) => {
    if (e.target === modal) {
      closeModal();
    }
  });

  // Close on Escape key
  modal.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      closeModal();
    }
  });

  initSizeFinder(modal);
}

// Foot-length → recommended size, from this product's own size chart.
// Rule: add a fixed room allowance, then recommend the smallest EU size whose
// insole length is at least (foot + allowance). Reads the chart embedded per
// product, so it always agrees with the table shown right below it.
const ROOM_MM = 7; // room added to the measured foot before matching the chart

function initSizeFinder(modal) {
  const input = modal.querySelector('#sg-calc-input');
  const btn = modal.querySelector('#sg-calc-btn');
  const out = modal.querySelector('#sg-calc-result');
  const chartEl = modal.querySelector('#sg-chart-data');
  if (!input || !out || !chartEl) return;

  let rows = [];
  try {
    rows = JSON.parse(chartEl.textContent || '[]')
      .filter((r) => r.foot_length_mm != null)
      .map((r) => ({ size: Number(r.eu_size), len: Number(r.foot_length_mm) }))
      .sort((a, b) => a.size - b.size);
  } catch {
    rows = [];
  }
  if (!rows.length) return;

  const fill = (tpl, map) => (tpl || '').replace(/\{(\w+)\}/g, (_, k) => map[k] ?? '');
  const num = (n) => String(n);

  // Some models run large lengthwise; treat their insoles as this many mm
  // longer so the finder recommends one size down (e.g. Keld).
  const offset = Number(out.dataset.fitoffset) || 0;

  function recommend() {
    const mm = parseFloat((input.value || '').replace(',', '.').trim());
    if (!isFinite(mm) || mm < 150 || mm > 400) {
      out.textContent = out.dataset.invalid;
      out.classList.remove('is-set');
      return;
    }
    const targetMm = mm + ROOM_MM;
    const smallest = rows[0];
    const largest = rows[rows.length - 1];

    let size;
    let note;
    if (targetMm > largest.len + offset) {
      size = largest.size;
      note = fill(out.dataset.toobig, { L: num(mm), S: num(largest.size) });
    } else {
      const hit = rows.find((r) => r.len + offset >= targetMm);
      size = hit.size;
      note =
        targetMm < smallest.len + offset
          ? fill(out.dataset.toosmall, { L: num(mm), S: num(smallest.size) })
          : fill(out.dataset.working, { L: num(mm), T: num(targetMm) });
    }

    out.innerHTML =
      `<span class="sg-calc__reco">${out.dataset.recommend}</span>` +
      `<span class="sg-calc__size">EU ${num(size)}</span>` +
      `<span class="sg-calc__note">${note}</span>`;
    out.classList.add('is-set');
  }

  btn?.addEventListener('click', recommend);
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      recommend();
    }
  });
  input.addEventListener('input', () => {
    if (input.value.trim() === '') {
      out.textContent = '';
      out.classList.remove('is-set');
    }
  });
}
