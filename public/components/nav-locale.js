import { closeDrawer } from './nav-drawer.js';

// Persistent SR announcer — lives on body, never inside swapped regions.
function getAnnouncer() {
  let el = document.getElementById('sr-announcer');
  if (!el) {
    el = document.createElement('div');
    el.id = 'sr-announcer';
    el.setAttribute('role', 'status');
    el.setAttribute('aria-live', 'polite');
    el.setAttribute('aria-atomic', 'true');
    el.className = 'sr-only';
    document.body.appendChild(el);
  }
  return el;
}

function announce(msg) {
  const el = getAnnouncer();
  el.textContent = '';
  // Small delay so screen readers catch the change
  setTimeout(() => {
    el.textContent = msg;
  }, 50);
}

function swapRegion(selector, doc) {
  const next = doc.querySelector(selector);
  const curr = document.querySelector(selector);
  if (next && curr) curr.replaceWith(next);
}

async function navigate(url) {
  closeDrawer();

  const res = await fetch(url, { headers: { 'X-Requested-With': 'fetch' } });
  if (!res.ok) {
    window.location.href = url;
    return;
  }

  const html = await res.text();
  const doc = new DOMParser().parseFromString(html, 'text/html');

  swapRegion('header', doc);
  swapRegion('main', doc);
  swapRegion('footer', doc);

  document.title = doc.title;
  document.documentElement.lang = doc.documentElement.lang;

  history.pushState({ url }, '', url);
  window.scrollTo(0, 0);

  // Move focus to main heading so keyboard users land somewhere sensible
  const h1 = document.querySelector('main h1');
  if (h1) {
    h1.setAttribute('tabindex', '-1');
    h1.focus({ preventScroll: true });
  }

  announce(document.title);
}

export function initNavLocale() {
  document.addEventListener('click', (e) => {
    const a = e.target.closest('.nav__lang a');
    if (!a) return;
    e.preventDefault();
    navigate(a.href);
  });

  window.addEventListener('popstate', (e) => {
    navigate(e.state?.url ?? location.href);
  });
}
