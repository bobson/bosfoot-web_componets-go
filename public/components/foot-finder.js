// <foot-finder> — HTML Web Component (custom element, light DOM, no shadow).
// A camera-based foot measurer: the user photographs their foot on an A4 sheet
// (or beside a bank card), taps the reference corners + heel/toe, and we recover
// the real foot length in mm via a homography — ALL on-device (nothing uploads).
// Static chrome is server-rendered inside the element; this class adds behaviour.
//
// Result is saved to localStorage under 'bosfoot_foot' so product pages can later
// pre-highlight the recommended size (Phase 2). The EU hint uses Bosfoot's own
// size-guide chart (foot mm -> EU) rather than a generic formula, so it matches
// the rest of the site.

const STORE_KEY = 'bosfoot_foot';

// ISO/IEC 7810 ID-1 bank card + A4, in mm.
const REFS = {
  a4: { w: 210, h: 297 },
  card: { w: 53.98, h: 85.6 },
};

// Bosfoot size-guide chart: foot length (mm) -> EU. Nearest wins. Mirrors the
// table in templates/pages/size-guide.html — keep them in sync.
const EU_CHART = [
  [230, 36],
  [237, 37],
  [244, 38],
  [251, 39],
  [258, 40],
  [265, 41],
  [272, 42],
  [279, 43],
  [286, 44],
  [293, 45],
  [300, 46],
];
function euFromFootMM(mm) {
  let best = EU_CHART[0];
  for (const row of EU_CHART) {
    if (Math.abs(row[0] - mm) < Math.abs(best[0] - mm)) best = row;
  }
  return best[1];
}

const MAX_CANVAS_DIM = 1500; // downscale phone photos so mobile canvas caps can't blank us

class FootFinder extends HTMLElement {
  connectedCallback() {
    this.i18n = this.readI18n();

    this.refType = 'a4';
    this.currentFoot = 'L';
    this.results = { L: null, R: null };
    this.img = null;
    this.phase = 'corners'; // 'corners' -> 'foot'
    this.corners = []; // image-space points
    this.footPts = []; // [heel, toe] image-space
    this.H = null; // homography image->mm
    this.dragIdx = -1;
    this.dragSet = null;

    this.canvas = this.q('#ff-canvas');
    this.ctx = this.canvas.getContext('2d');

    this.wire();
    this.showStep(1);
  }

  // ---------- helpers ----------
  q(sel) {
    return this.querySelector(sel);
  }
  readI18n() {
    try {
      return JSON.parse(this.q('#ff-i18n').textContent);
    } catch {
      return {};
    }
  }
  t(key, params) {
    let s = this.i18n[key] ?? key;
    if (params) for (const k in params) s = s.replaceAll(`{${k}}`, params[k]);
    return s;
  }
  refName() {
    return this.t(this.refType === 'a4' ? 'ref.a4' : 'ref.card');
  }

  wire() {
    this.q('#ff-refA4').onclick = () => this.setRef('a4');
    this.q('#ff-refCard').onclick = () => this.setRef('card');
    this.q('#ff-tabL').onclick = () => this.setFoot('L');
    this.q('#ff-tabR').onclick = () => this.setFoot('R');

    this.q('#ff-btnPhoto').onclick = () => this.q('#ff-file').click();
    this.q('#ff-file').onchange = (e) => this.onFile(e);
    this.q('#ff-btnRetake').onclick = () => this.showStep(1);
    this.q('#ff-btnUndo').onclick = () => this.onUndo();
    this.q('#ff-btnNext').onclick = () => this.onNext();
    this.q('#ff-btnOtherFoot').onclick = () => {
      this.setFoot(this.currentFoot === 'L' ? 'R' : 'L');
      this.showStep(1);
    };
    this.q('#ff-btnRestart').onclick = () => this.onRestart();

    const c = this.canvas;
    c.addEventListener('mousedown', (e) => this.pointerDown(e));
    c.addEventListener('mousemove', (e) => this.pointerMove(e));
    window.addEventListener('mouseup', () => this.pointerUp());
    c.addEventListener('touchstart', (e) => this.pointerDown(e), { passive: false });
    c.addEventListener('touchmove', (e) => this.pointerMove(e), { passive: false });
    c.addEventListener('touchend', () => this.pointerUp());
  }

  // ---------- navigation / tabs ----------
  showStep(n) {
    this.q('#ff-step1').hidden = n !== 1;
    this.q('#ff-step2').hidden = n !== 2 && n !== 3;
    this.q('#ff-step4').hidden = n !== 4;
    for (const i of [1, 2, 3, 4]) {
      this.q('#ff-st' + i).classList.toggle('on', i <= n);
    }
  }
  setRef(t) {
    this.refType = t;
    this.q('#ff-refA4').classList.toggle('active', t === 'a4');
    this.q('#ff-refCard').classList.toggle('active', t === 'card');
    this.q('#ff-howtoA4').hidden = t !== 'a4';
    this.q('#ff-howtoCard').hidden = t !== 'card';
  }
  setFoot(f) {
    this.currentFoot = f;
    this.q('#ff-tabL').classList.toggle('active', f === 'L');
    this.q('#ff-tabR').classList.toggle('active', f === 'R');
  }

  // ---------- photo ----------
  onFile(e) {
    const file = e.target.files[0];
    if (!file) return;
    const url = URL.createObjectURL(file);
    const im = new Image();
    im.onload = () => {
      this.img = im;
      this.corners = [];
      this.footPts = [];
      this.H = null;
      this.phase = 'corners';
      this.setupCanvas();
      this.updateMarkUI();
      this.showStep(2);
      this.draw();
      URL.revokeObjectURL(url);
    };
    im.src = url;
    e.target.value = '';
  }

  onUndo() {
    if (this.phase === 'corners' && this.corners.length) this.corners.pop();
    else if (this.phase === 'foot' && this.footPts.length) this.footPts.pop();
    else if (this.phase === 'foot') this.phase = 'corners';
    this.updateMarkUI();
    this.draw();
  }

  onNext() {
    if (this.phase === 'corners' && this.corners.length === 4) {
      this.H = this.computeHomography(this.sortCorners(this.corners));
      if (!this.H) {
        alert(this.t('err.corners'));
        return;
      }
      this.phase = 'foot';
      this.footPts = [];
      this.updateMarkUI();
      this.draw();
      this.showStep(3);
    } else if (this.phase === 'foot' && this.footPts.length === 2) {
      this.finish();
    }
  }

  onRestart() {
    this.results.L = null;
    this.results.R = null;
    this.q('#ff-tabL').classList.remove('done');
    this.q('#ff-tabR').classList.remove('done');
    this.setFoot('L');
    this.showStep(1);
  }

  updateMarkUI() {
    const hint = this.q('#ff-hint span:last-child');
    const dot = this.q('#ff-hint .dot');
    if (this.phase === 'corners') {
      this.q('#ff-markTitle').textContent = this.t('mark.cornersTitle', { ref: this.refName() });
      let msg = this.t('mark.cornersHint', { ref: this.refName(), n: this.corners.length });
      if (this.refType === 'card') msg += ' ' + this.t('mark.cardCorners');
      hint.textContent = msg;
      dot.style.background = 'var(--color-star)';
      this.q('#ff-btnNext').disabled = this.corners.length !== 4;
      this.q('#ff-btnNext').textContent = this.t('btn.continue');
      this.q('#ff-live').hidden = true;
    } else {
      const key =
        this.footPts.length === 0
          ? 'mark.heel'
          : this.footPts.length === 1
            ? 'mark.toe'
            : 'mark.adjust';
      this.q('#ff-markTitle').textContent = this.t(key);
      hint.textContent = this.t('mark.footHint', { n: this.footPts.length });
      dot.style.background = '#4FA46A';
      this.q('#ff-btnNext').disabled = this.footPts.length !== 2;
      this.q('#ff-btnNext').textContent = this.t('btn.showSize');
      this.updateLive();
    }
  }

  // ---------- canvas ----------
  setupCanvas() {
    // Downscale big phone photos: mobile canvases have a max area/side; a raw
    // 12MP image can blank the canvas. All point math is in canvas-pixel space
    // and the homography is scale-invariant, so accuracy is unaffected.
    const scale = Math.min(1, MAX_CANVAS_DIM / Math.max(this.img.width, this.img.height));
    this.canvas.width = Math.round(this.img.width * scale);
    this.canvas.height = Math.round(this.img.height * scale);
    this.canvas.style.width = '100%';
  }

  evtToImg(e) {
    const r = this.canvas.getBoundingClientRect();
    const t = e.touches ? e.touches[0] : e;
    return {
      x: ((t.clientX - r.left) / r.width) * this.canvas.width,
      y: ((t.clientY - r.top) / r.height) * this.canvas.height,
    };
  }

  nearPoint(p, list) {
    const r = this.canvas.getBoundingClientRect();
    const tol = (30 * this.canvas.width) / r.width; // ~30 css px
    for (let i = 0; i < list.length; i++) {
      if (Math.hypot(list[i].x - p.x, list[i].y - p.y) < tol) return i;
    }
    return -1;
  }

  pointerDown(e) {
    if (!this.img) return;
    e.preventDefault();
    const p = this.evtToImg(e);
    const list = this.phase === 'corners' ? this.corners : this.footPts;
    const idx = this.nearPoint(p, list);
    if (idx >= 0) {
      this.dragIdx = idx;
      this.dragSet = list;
      return;
    }
    const max = this.phase === 'corners' ? 4 : 2;
    if (list.length < max) {
      list.push(p);
      this.dragIdx = list.length - 1;
      this.dragSet = list;
    }
    this.updateMarkUI();
    this.draw();
  }
  pointerMove(e) {
    if (this.dragIdx < 0) return;
    e.preventDefault();
    this.dragSet[this.dragIdx] = this.evtToImg(e);
    if (this.phase === 'foot') this.updateLive();
    this.draw();
  }
  pointerUp() {
    this.dragIdx = -1;
    this.dragSet = null;
    this.updateMarkUI();
  }

  draw() {
    if (!this.img) return;
    const { ctx, canvas } = this;
    ctx.drawImage(this.img, 0, 0, canvas.width, canvas.height);
    const lw = Math.max(2, canvas.width / 300);
    if (this.corners.length > 1) {
      const cs = this.corners.length === 4 ? this.sortCorners(this.corners) : this.corners;
      ctx.beginPath();
      ctx.moveTo(cs[0].x, cs[0].y);
      for (let i = 1; i < cs.length; i++) ctx.lineTo(cs[i].x, cs[i].y);
      if (this.corners.length === 4) ctx.closePath();
      ctx.strokeStyle = 'rgba(232,179,48,.9)';
      ctx.lineWidth = lw;
      ctx.stroke();
    }
    this.corners.forEach((c, i) => this.dot(c, '#E8B330', i + 1));
    if (this.phase === 'foot') {
      if (this.footPts.length === 2) {
        ctx.beginPath();
        ctx.moveTo(this.footPts[0].x, this.footPts[0].y);
        ctx.lineTo(this.footPts[1].x, this.footPts[1].y);
        ctx.strokeStyle = 'rgba(255,255,255,.95)';
        ctx.lineWidth = lw;
        ctx.setLineDash([lw * 3, lw * 2]);
        ctx.stroke();
        ctx.setLineDash([]);
      }
      this.footPts.forEach((c, i) => this.dot(c, '#4FA46A', i === 0 ? 'H' : 'T'));
    }
  }
  dot(p, color, label) {
    const { ctx, canvas } = this;
    const r = Math.max(9, canvas.width / 70);
    ctx.beginPath();
    ctx.arc(p.x, p.y, r, 0, 7);
    ctx.fillStyle = color;
    ctx.fill();
    ctx.lineWidth = r / 4;
    ctx.strokeStyle = '#fff';
    ctx.stroke();
    ctx.fillStyle = '#fff';
    ctx.font = 'bold ' + r * 1.1 + 'px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(String(label), p.x, p.y);
  }

  // ---------- geometry (unchanged from the reference math) ----------
  sortCorners(pts) {
    const cx = pts.reduce((s, p) => s + p.x, 0) / 4;
    const cy = pts.reduce((s, p) => s + p.y, 0) / 4;
    const s = [...pts].sort(
      (a, b) => Math.atan2(a.y - cy, a.x - cx) - Math.atan2(b.y - cy, b.x - cx),
    );
    let start = 0;
    let best = 1e18;
    s.forEach((p, i) => {
      const v = p.x + p.y;
      if (v < best) {
        best = v;
        start = i;
      }
    });
    return [0, 1, 2, 3].map((i) => s[(start + i) % 4]);
  }

  computeHomography(c) {
    const d01 = Math.hypot(c[1].x - c[0].x, c[1].y - c[0].y);
    const d12 = Math.hypot(c[2].x - c[1].x, c[2].y - c[1].y);
    const portrait = d12 >= d01;
    const R = REFS[this.refType];
    const W = portrait ? R.w : R.h;
    const Hh = portrait ? R.h : R.w;
    const dst = [
      { x: 0, y: 0 },
      { x: W, y: 0 },
      { x: W, y: Hh },
      { x: 0, y: Hh },
    ];
    const A = [];
    const b = [];
    for (let i = 0; i < 4; i++) {
      const { x, y } = c[i];
      const { x: X, y: Y } = dst[i];
      A.push([x, y, 1, 0, 0, 0, -X * x, -X * y]);
      b.push(X);
      A.push([0, 0, 0, x, y, 1, -Y * x, -Y * y]);
      b.push(Y);
    }
    const h = this.solve(A, b);
    if (!h) return null;
    return [h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1];
  }

  solve(A, b) {
    const n = 8;
    const M = A.map((row, i) => [...row, b[i]]);
    for (let col = 0; col < n; col++) {
      let piv = col;
      for (let r = col + 1; r < n; r++) {
        if (Math.abs(M[r][col]) > Math.abs(M[piv][col])) piv = r;
      }
      if (Math.abs(M[piv][col]) < 1e-10) return null;
      [M[col], M[piv]] = [M[piv], M[col]];
      for (let r = 0; r < n; r++) {
        if (r === col) continue;
        const f = M[r][col] / M[col][col];
        for (let k = col; k <= n; k++) M[r][k] -= f * M[col][k];
      }
    }
    return M.map((row, i) => row[n] / row[i]);
  }

  toMM(p) {
    const [a, b2, c2, d, e, f, g, h] = this.H;
    const w = g * p.x + h * p.y + 1;
    return { x: (a * p.x + b2 * p.y + c2) / w, y: (d * p.x + e * p.y + f) / w };
  }
  footLenMM() {
    if (!this.H || this.footPts.length < 2) return null;
    const p1 = this.toMM(this.footPts[0]);
    const p2 = this.toMM(this.footPts[1]);
    return Math.hypot(p2.x - p1.x, p2.y - p1.y);
  }

  updateLive() {
    const len = this.footLenMM();
    const el = this.q('#ff-live');
    if (len) {
      el.textContent = (len / 10).toFixed(1) + ' cm';
      el.hidden = false;
    } else {
      el.hidden = true;
    }
  }

  // ---------- result ----------
  finish() {
    const len = this.footLenMM();
    if (!len || len < 150 || len > 400) {
      alert(this.t('err.measure', { cm: len ? (len / 10).toFixed(1) : '–' }));
      return;
    }
    this.results[this.currentFoot] = len;
    this.q('#ff-tab' + this.currentFoot).classList.add('done');

    const fit = len + 12; // barefoot toe room
    const eu = euFromFootMM(len);
    this.q('#ff-resFootLabel').textContent = this.t(
      this.currentFoot === 'L' ? 'foot.left' : 'foot.right',
    );
    this.q('#ff-resLen').textContent = (len / 10).toFixed(1) + ' cm';
    this.q('#ff-resEU').textContent = 'EU ' + eu;
    this.q('#ff-resFit').textContent = (fit / 10).toFixed(1) + ' cm';

    const other = this.currentFoot === 'L' ? this.results.R : this.results.L;
    if (other) {
      const longer = Math.max(this.results.L, this.results.R);
      const lf = longer === this.results.L ? this.t('foot.left') : this.t('foot.right');
      this.q('#ff-resOther').textContent = this.t('res.bothFeet', {
        cm: (other / 10).toFixed(1),
        foot: lf,
        eu: euFromFootMM(longer),
      });
    } else {
      this.q('#ff-resOther').textContent = this.t('res.oneFoot');
    }

    this.save(len, eu);
    this.showStep(4);
  }

  // Persist for Phase 2 (product-page size hint). Browser-only, like the cart.
  save(len, eu) {
    let store = {};
    try {
      store = JSON.parse(localStorage.getItem(STORE_KEY) || '{}');
    } catch {
      store = {};
    }
    store[this.currentFoot] = Math.round(len);
    const longer = Math.max(this.results.L || 0, this.results.R || 0);
    store.eu = euFromFootMM(longer);
    store.updated = Date.now();
    try {
      localStorage.setItem(STORE_KEY, JSON.stringify(store));
    } catch {
      /* private mode / quota — the on-screen result still works */
    }
  }
}

customElements.define('foot-finder', FootFinder);

export function initFootFinder() {
  // The custom element self-initialises on connect.
}
