// <review-form> — an HTML Web Component (custom element, light DOM, no shadow).
// Wires the star + fit pickers, validates (a star rating is required, fit is
// optional), POSTs to /api/reviews with the CSRF header, then swaps in the
// thank-you panel. The static chrome is server-rendered inside the element.

class ReviewForm extends HTMLElement {
  connectedCallback() {
    this.token = this.dataset.token || '';
    this.maxPhotos = Number(this.dataset.maxPhotos) || 3;
    this.errRating = this.dataset.errorRating || 'Please choose a star rating';
    this.errPhotos = this.dataset.errorPhotos || `You can add up to ${this.maxPhotos} photos.`;
    this.errGeneric = this.dataset.errorGeneric || 'Something went wrong. Please try again.';

    this.form = this.querySelector('[data-review-form]');
    this.starsEl = this.querySelector('[data-stars]');
    this.ratingInput = this.querySelector('[data-rating-input]');
    this.fitEl = this.querySelector('[data-fit]');
    this.fitInput = this.querySelector('[data-fit-input]');
    this.photosInput = this.querySelector('[data-photos-input]');
    this.previewEl = this.querySelector('[data-photos-preview]');
    this.errorEl = this.querySelector('[data-review-error]');
    this.submitBtn = this.querySelector('[data-review-submit]');
    this.thanksEl = this.querySelector('[data-review-thanks]');
    if (!this.form) return;

    this.rating = 0;
    this.photos = [];
    this.bindStars();
    this.bindFit();
    this.bindPhotos();
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

  bindPhotos() {
    if (!this.photosInput) return;
    this.photosInput.addEventListener('change', () => {
      const picked = [...this.photosInput.files];
      if (picked.length > this.maxPhotos) this.showError(this.errPhotos);
      else this.errorEl.hidden = true;
      // Keep at most maxPhotos; the server also caps and re-encodes.
      this.photos = picked.slice(0, this.maxPhotos);
      this.renderPreviews();
    });
  }

  renderPreviews() {
    if (!this.previewEl) return;
    // Free the previous object URLs before replacing them.
    this.previewEl.querySelectorAll('img').forEach((img) => URL.revokeObjectURL(img.src));
    this.previewEl.textContent = '';
    for (const file of this.photos) {
      const img = document.createElement('img');
      img.className = 'review-photos__thumb';
      img.alt = '';
      img.src = URL.createObjectURL(file);
      this.previewEl.appendChild(img);
    }
  }

  async onSubmit(e) {
    e.preventDefault();
    this.errorEl.hidden = true;

    if (this.rating < 1) {
      this.showError(this.errRating);
      return;
    }
    if (!this.form.reportValidity()) return;

    // Multipart so photos ride along. Do NOT set Content-Type — the browser adds
    // the multipart boundary itself; setting it manually breaks the upload.
    const fd = new FormData();
    fd.append('token', this.token);
    fd.append('rating', String(this.rating));
    fd.append('fit', this.fitInput.value || '');
    fd.append('author_name', (this.form.author_name?.value || '').trim());
    fd.append('body', (this.form.body?.value || '').trim());
    for (const file of this.photos) fd.append('photos', file);

    this.setBusy(true);
    try {
      const csrfToken = document.cookie.match(/(?:^|;\s*)_csrf=([^;]+)/)?.[1] ?? '';
      const res = await fetch('/api/reviews', {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrfToken },
        body: fd,
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
