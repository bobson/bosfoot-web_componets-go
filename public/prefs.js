// Site preferences: cookie helpers + consent state + Meta Pixel bootstrap.
//
// NOTE ON NAMING: this file and its component are deliberately named "prefs",
// NOT "consent"/"cookie-*". Ad blockers (EasyList) network-block filenames like
// cookie-consent.js, so a consent-named module would be blocked for exactly the
// privacy-conscious visitors it targets — and because app.js STATICALLY imports
// this, a blocked module would take down all interactivity. Neutral names avoid
// both. (The Meta Pixel's own connect.facebook.net request is still blocked by
// such users, which is fine — no consent, no pixel.)
//
// Imported statically (no ?v= stamp), but every un-fingerprinted .js is served
// no-store (internal/routes/routes.go), so this can't go stale in the FB webview.

const CONSENT_COOKIE = 'bosfoot_consent';

export function getCookie(name) {
  return document.cookie
    .split('; ')
    .find((row) => row.startsWith(`${name}=`))
    ?.split('=')[1];
}

export function setCookie(name, value, days) {
  const expires = new Date(Date.now() + days * 864e5).toUTCString();
  const secure = location.protocol === 'https:' ? '; Secure' : '';
  document.cookie = `${name}=${value}; Path=/; Expires=${expires}; SameSite=Lax${secure}`;
}

// Returns 'accepted', 'declined', or undefined (no choice made yet).
export function consentState() {
  return getCookie(CONSENT_COOKIE);
}

export function storeConsent(accepted) {
  // 180 days — after that the cookie expires and we re-ask, per ePrivacy guidance.
  setCookie(CONSENT_COOKIE, accepted ? 'accepted' : 'declined', 180);
}

// Install the Meta Pixel: the standard base stub (which QUEUES events until
// fbevents.js loads), then init + PageView. Idempotent — the stub's own guard
// plus the window.fbq check make a second call a no-op, so app.js and the banner
// component can both call it safely. Reads the pixel ID from the inert <meta> tag
// in <head>; a no-op when it's absent (META_PIXEL_ID unset).
export function bootstrapPixel() {
  const id = document.querySelector('meta[name="meta-pixel-id"]')?.content;
  if (!id || window.fbq) return;

  (function (f, b, e, v, n, t, s) {
    if (f.fbq) return;
    n = f.fbq = function () {
      n.callMethod ? n.callMethod.apply(n, arguments) : n.queue.push(arguments);
    };
    if (!f._fbq) f._fbq = n;
    n.push = n;
    n.loaded = true;
    n.version = '2.0';
    n.queue = [];
    t = b.createElement(e);
    t.async = true;
    t.src = v;
    s = b.getElementsByTagName(e)[0];
    s.parentNode.insertBefore(t, s);
  })(window, document, 'script', 'https://connect.facebook.net/en_US/fbevents.js');

  window.fbq('init', id);
  window.fbq('track', 'PageView');
}
