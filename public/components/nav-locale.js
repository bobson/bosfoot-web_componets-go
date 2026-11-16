import { setCookie } from '../prefs.js';

// Remember the language currently being viewed so the "/" redirect (server-side,
// internal/routes/routes.go) can send a returning visitor straight to it instead
// of always defaulting to /mk. Non-sensitive, so JS-set (not HttpOnly) is fine.
//
// The language links themselves are plain <a href> full page loads. They used
// to be intercepted and the header/main/footer swapped in via fetch, but the
// other components bind once at page load (product-detail, size guide, search,
// scroll-reveal…), so the swapped-in page was dead: nothing clickable and
// JS-revealed content (e.g. the reservation note) stayed hidden. A real
// navigation re-runs everything; language switches are rare, so it costs nothing.
function rememberLocale() {
  const lang = document.documentElement.lang;
  if (lang) setCookie('bosfoot_locale', lang, 365);
}

export function initNavLocale() {
  rememberLocale(); // persist the language of the page we landed on
}
