// <review-form> — an HTML Web Component (custom element, light DOM, no shadow).
// Wires the star + fit pickers, validates (a star rating is required, fit is
// optional), POSTs to /api/reviews with the CSRF header, then swaps in the
// thank-you panel. The static chrome is server-rendered inside the element.

class ReviewForm extends HTMLElement {
  connectedCallback() {
    this.token = this.dataset.token || '';
    this.errRating = this.dataset.errorRating || 'Please choose a star rating';
    this.errGeneric = this.dataset.errorGeneric || 'Something went wrong. Please try again.';

    this.form = this.querySelector('[data-review-form]');
    this.starsEl = this.querySelector('[data-stars]');
    this.ratingInput = this.querySelector('[data-rating-input]');
    this.fitEl = this.querySelector('[data-fit]');
    this.fitInput = this.querySelector('[data-fit-input]');
    this.errorEl = this.querySelector('[data-review-error]');
    this.submitBtn = this.querySelector('[data-review-submit]');
    this.thanksEl = this.querySelector('[data-review-thanks]');
    if (!this.form) return;

    this.rating = 0;
    this.bindStars();
    this.bindFit();
    this.form.addEventListener('submit', (e) => this.onSubmit(e));
  }

  bindStars() {
    const btns = [...this.starsEl.querySelectorAll('[data-star]')];
    const paint = (upTo) => {
      btns.forEach((b) => {
        const v = Number(b.dataset.star);
        b.classList.toggle('is-on', v <= upTo);
      });
    };
    btns.forEach((btn) => {
      const v = Number(btn.dataset.star);
      btn.addEventListener('mouseenter', () => paint(v));
      btn.addEventListener('focus', () => paint(v));
      btn.addEventListener('click', () => {
        this.rating = v;
        this.ratingInput.value = String(v);
        btns.forEach((b) => b.setAttribute('aria-checked', b === btn ? 'true' : 'false'));
        paint(v);
        this.errorEl.hidden = true;
      });
    });
    // Restore the chosen fill when the pointer leaves the row.
    this.starsEl.addEventListener('mouseleave', () => paint(this.rating));
  }

  bindFit() {
    const btns = [...this.fitEl.querySelectorAll('[data-fit-value]')];
    btns.forEach((btn) => {
      btn.addEventListener('click', () => {
        const already = btn.classList.contains('is-on');
        btns.forEach((b) => {
          b.classList.remove('is-on');
          b.setAttribute('aria-checked', 'false');
        });
        // Clicking the selected option again clears it (fit is optional).
        if (already) {
          this.fitInput.value = '';
        } else {
          btn.classList.add('is-on');
          btn.setAttribute('aria-checked', 'true');
          this.fitInput.value = btn.dataset.fitValue;
        }
      });
    });
  }

  async onSubmit(e) {
    e.preventDefault();
    this.errorEl.hidden = true;

    if (this.rating < 1) {
      this.showError(this.errRating);
      return;
    }
    if (!this.form.reportValidity()) return;

    const fd = new FormData(this.form);
    const fitRaw = (fd.get('fit') || '').toString();
    const payload = {
      token: this.token,
      rating: this.rating,
      fit: fitRaw === '' ? null : Number(fitRaw),
      author_name: (fd.get('author_name') || '').toString().trim(),
      body: (fd.get('body') || '').toString().trim(),
    };

    this.setBusy(true);
    try {
      const csrfToken = document.cookie.match(/(?:^|;\s*)_csrf=([^;]+)/)?.[1] ?? '';
      const res = await fetch('/api/reviews', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        this.showError(this.errGeneric);
        this.setBusy(false);
        return;
      }
      this.onSuccess();
    } catch {
      this.showError(this.errGeneric);
      this.setBusy(false);
    }
  }

  showError(msg) {
    this.errorEl.textContent = msg;
    this.errorEl.hidden = false;
  }

  setBusy(busy) {
    this.submitBtn.disabled = busy;
    this.submitBtn.textContent = busy
      ? this.submitBtn.dataset.labelSubmitting
      : this.submitBtn.dataset.labelSubmit;
  }

  onSuccess() {
    this.form.hidden = true;
    if (this.thanksEl) this.thanksEl.hidden = false;
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }
}

customElements.define('review-form', ReviewForm);

export function initReviewForm() {
  // Web component self-initialises on connection.
}
