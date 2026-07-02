import { bootstrapPixel, consentState, storeConsent } from '../prefs.js';

// Consent banner UI. Named "prefs" (not "cookie-consent") so ad blockers don't
// network-block it — see prefs.js. The static chrome is server-rendered (hidden)
// inside #prefs-bar and only when a pixel is configured; this shows it to
// undecided visitors and wires Accept/Decline. Loading the pixel for already-
// accepted visitors happens synchronously in app.js (before the component imports
// resolve) so no product-page ViewContent is lost to a bootstrap race.
export function initPrefsUI() {
  const bar = document.getElementById('prefs-bar');
  if (!bar) return; // no META_PIXEL_ID → nothing to consent to

  const state = consentState();
  if (state === 'accepted') {
    bootstrapPixel(); // idempotent; app.js already did this on load
    return;
  }
  if (state === 'declined') return;

  bar.hidden = false;

  bar.querySelector('[data-prefs="accept"]')?.addEventListener('click', () => {
    storeConsent(true);
    bootstrapPixel();
    bar.hidden = true;
  });

  bar.querySelector('[data-prefs="decline"]')?.addEventListener('click', () => {
    storeConsent(false);
    bar.hidden = true;
  });
}
