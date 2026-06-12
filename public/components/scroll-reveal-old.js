// scroll-reveal.js — lightweight content-page motion: reveal-on-enter,
// stat count-up, and the transition-timeline fill. Everything degrades to the
// plain SSR content under reduced motion or without IntersectionObserver, so
// nothing depends on JS to be readable.

const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
const hasIO = 'IntersectionObserver' in window;

// ── Reveal on scroll ──────────────────────────────────
(function reveal() {
  const els = document.querySelectorAll('[data-reveal]');
  if (!els.length) return;
  if (reduce || !hasIO) {
    els.forEach((el) => el.classList.add('is-visible'));
    return;
  }
  const io = new IntersectionObserver(
    (entries, obs) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add('is-visible');
          obs.unobserve(e.target);
        }
      });
    },
    { threshold: 0.12, rootMargin: '0px 0px -8% 0px' },
  );
  els.forEach((el) => io.observe(el));
})();

// ── Stat count-up ─────────────────────────────────────
(function countUp() {
  const nums = document.querySelectorAll('[data-count]');
  if (!nums.length || reduce || !hasIO) return; // leave SSR numbers as-is

  const run = (el) => {
    const target = parseInt(el.dataset.count, 10) || 0;
    const suffix = el.dataset.countSuffix || '';
    const dur = 1100;
    const start = performance.now();
    const tick = (now) => {
      const p = Math.min((now - start) / dur, 1);
      const eased = 1 - Math.pow(1 - p, 3);
      el.textContent = Math.round(target * eased) + suffix;
      if (p < 1) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  };

  const io = new IntersectionObserver(
    (entries, obs) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          run(e.target);
          obs.unobserve(e.target);
        }
      });
    },
    { threshold: 0.6 },
  );
  nums.forEach((n) => io.observe(n));
})();

// ── Transition timeline fill ──────────────────────────
(function timeline() {
  const tl = document.querySelector('[data-timeline]');
  if (!tl) return;
  if (reduce || !hasIO) {
    tl.classList.add('is-active'); // show the full rail immediately
    return;
  }
  const io = new IntersectionObserver(
    (entries, obs) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          tl.classList.add('is-active');
          obs.unobserve(tl);
        }
      });
    },
    { threshold: 0.25 },
  );
  io.observe(tl);
})();
