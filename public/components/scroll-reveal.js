/* Scroll reveal + stat count-up.
   Place at /components/scroll-reveal.js (already linked from templates).
   - [data-reveal]            fade-rise on scroll, auto-staggered siblings
   - [data-count="26"]        counts up when revealed
   - [data-count-suffix="+"]  optional suffix appended after the number
   Respects prefers-reduced-motion; without JS content stays visible. */

const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;

function countUp(el) {
  const target = parseInt(el.dataset.count, 10);
  const suffix = el.dataset.countSuffix ?? '';
  if (!Number.isFinite(target)) return;
  const dur = 900;
  const start = performance.now();
  function tick(now) {
    const p = Math.min((now - start) / dur, 1);
    const eased = 1 - Math.pow(1 - p, 3); // ease-out cubic
    el.textContent = Math.round(eased * target) + (p === 1 ? suffix : '');
    if (p < 1) requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);
}

function activate(el) {
  el.classList.add('is-visible');
  if (!reduced) el.querySelectorAll('[data-count]').forEach(countUp);
}

export function initScrollReveal() {
  // Hidden states in CSS apply only under html.js-reveal-ready, so content is
  // always visible if this script never runs or hasn't started yet.
  if (!reduced && 'IntersectionObserver' in window) {
    document.documentElement.classList.add('js-reveal-ready');
  }

  const els = document.querySelectorAll('[data-reveal]');

  if (reduced || !('IntersectionObserver' in window)) {
    els.forEach((el) => el.classList.add('is-visible'));
    return;
  }

  const io = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        io.unobserve(entry.target);
        activate(entry.target);
      }
    },
    { threshold: 0.12, rootMargin: '0px 0px -8% 0px' },
  );

  function scan(container = document) {
    const newEls = container.querySelectorAll('[data-reveal]:not(.is-visible)');
    newEls.forEach((el) => {
      // Stagger siblings that reveal together
      const peers = el.parentElement
        ? [...el.parentElement.children].filter((c) => c.hasAttribute('data-reveal'))
        : [el];
      const idx = peers.indexOf(el);
      if (idx > 0) el.style.transitionDelay = `${Math.min(idx, 5) * 70}ms`;
      io.observe(el);
    });
  }

  scan();

  // Handle dynamically injected content (language switching)
  const mo = new MutationObserver((mutations) => {
    for (const m of mutations) {
      for (const node of m.addedNodes) {
        if (node.nodeType !== 1) continue;
        if (node.hasAttribute?.('data-reveal')) scan(node.parentElement);
        else if (node.querySelectorAll) scan(node);
      }
    }
  });
  mo.observe(document.body, { childList: true, subtree: true });

  // Failsafe: the entrance animation must never gate the content.
  // still hidden a moment after load — below the fold and not yet scrolled to,
  // or an observer that didn't fire — is shown anyway. Guarantees text is
  // always visible; the scroll reveal stays a bonus for what's near the top.
  const revealAll = () => els.forEach((el) => el.classList.add('is-visible'));
  addEventListener('load', () => setTimeout(revealAll, 1000));
}
