import { dismissNotice, noticeDismissed } from '../prefs.js';

// Informational cookie notice. The Meta Pixel loads for every visitor in app.js;
// this component just shows a one-time "we use cookies" notice with a single
// dismiss button and remembers that it's been dismissed. Named "prefs" (not
// "cookie-consent") so ad blockers don't network-block it — see prefs.js.
export function initPrefsUI() {
  const bar = document.getElementById('prefs-bar');
  if (!bar) return; // no META_PIXEL_ID → no notice
  if (noticeDismissed()) return; // already seen on a previous visit

  bar.hidden = false;

  bar.querySelector('[data-prefs="dismiss"]')?.addEventListener('click', () => {
    dismissNotice();
    bar.hidden = true;
  });
}
