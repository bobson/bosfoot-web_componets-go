import { closeDrawer } from './nav-drawer.js';
import { setCookie } from '../prefs.js';

// Remember the language currently being viewed so the "/" redirect (server-side,
// internal/routes/routes.go) can send a returning visitor straight to it instead
// of always defaulting to /mk. Non-sensitive, so JS-set (not HttpOnly) is fine.
function rememberLocale() {
  const lang = document.documentElement.lang;
  if (lang) setCookie('bosfoot_locale', lang, 365);
}

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
  rememberLocale(); // the SPA swap changed the language; persist the new choice

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
  rememberLocale(); // persist the language of the page we landed on

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
