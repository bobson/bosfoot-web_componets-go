// Site preferences: cookie helpers + consent state + Meta Pixel bootstrap.
//
// NOTE ON NAMING: this file and its component are deliberately named "prefs",
// NOT "consent"/"cookie-*". Ad blockers (EasyList) network-block filenames like
// cookie-consent.js, so a consent-named module would be blocked for exactly the
// privacy-conscious visitors it targets — and because app.js STATICALLY imports
// this, a blocked module would take down all interactivity. Neutral names avoid
// both. (The Meta Pixel's own connect.facebook.net request is still blocked by
// such users, which is fine — those visitors simply aren't measured.)
//
// Imported statically (no ?v= stamp), but every un-fingerprinted .js is served
// no-store (internal/routes/routes.go), so this can't go stale in the FB webview.

const NOTICE_COOKIE = 'bosfoot_consent';

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

// The pixel loads for every visitor (see app.js); the banner is an informational
// notice, so we only track whether it's been dismissed. Any stored value —
// including a legacy 'accepted'/'declined' — counts as already seen.
export function noticeDismissed() {
  return !!getCookie(NOTICE_COOKIE);
}

export function dismissNotice() {
  setCookie(NOTICE_COOKIE, 'seen', 365);
}

// Install the Meta Pixel. Two-phase so tracking is lossless AND off the critical
// path: the fbq stub + init + PageView run synchronously (cheap — they only push
// onto a queue), so events fired during page init (e.g. ViewContent) are captured
// even before the library exists. The heavy 110 KiB fbevents.js — which caused the
// long tasks inside the LCP window — is deferred to the first interaction or
// window 'load' (see below) and flushes the queued events when it loads.
// Idempotent (window.fbq guard), so app.js and the banner component can both call
// it. No-op when META_PIXEL_ID is unset (no <meta> tag).
export function bootstrapPixel() {
  const id = document.querySelector('meta[name="meta-pixel-id"]')?.content;
  if (!id || window.fbq) return;

  // Phase 1 (synchronous): the standard fbq stub that queues calls until the
  // library loads, then init + PageView — all just enqueue, no network yet.
  const n = (window.fbq = function () {
    n.callMethod ? n.callMethod.apply(n, arguments) : n.queue.push(arguments);
  });
  if (!window._fbq) window._fbq = n;
  n.push = n;
  n.loaded = true;
  n.version = '2.0';
  n.queue = [];
  window.fbq('init', id);
  window.fbq('track', 'PageView');

  // Phase 2 (deferred): download fbevents.js AFTER the hero paints, but reliably
  // early — on the FIRST of a user interaction or window 'load' — so the queued
  // PageView/ViewContent still send for visitors who bounce quickly. (A long idle
  // timeout would risk losing those top-of-funnel events on slow mobile, which is
  // exactly the ad's landing traffic.) Conversions are unaffected — they fire
  // post-interaction, by which point the library is already loaded.
  const loadLib = () => {
    if (document.getElementById('fb-pixel-lib')) return;
    const t = document.createElement('script');
    t.async = true;
    t.id = 'fb-pixel-lib';
    t.src = 'https://connect.facebook.net/en_US/fbevents.js';
    document.head.appendChild(t);
  };
  for (const ev of ['pointerdown', 'keydown', 'touchstart', 'scroll']) {
    window.addEventListener(ev, loadLib, { once: true, passive: true });
  }
  if (document.readyState === 'complete') {
    setTimeout(loadLib, 800);
  } else {
    window.addEventListener('load', () => setTimeout(loadLib, 800), { once: true });
  }
}
