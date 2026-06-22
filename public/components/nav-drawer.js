// Document-level delegation — survives header replacement on locale swap.

const FOCUSABLE = 'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])';

function getEls() {
  return {
    drawer: document.getElementById('nav-drawer'),
    overlay: document.getElementById('nav-overlay'),
    openBtn: document.getElementById('nav-open'),
    closeBtn: document.getElementById('nav-close'),
  };
}

function trapFocus(e) {
  if (e.key !== 'Tab') return;
  const { drawer } = getEls();
  const focusable = [...drawer.querySelectorAll(FOCUSABLE)];
  const first = focusable[0];
  const last = focusable[focusable.length - 1];

  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault();
    first.focus();
  }
}

export function openDrawer() {
  const { drawer, openBtn, closeBtn } = getEls();
  if (!drawer) return; // Guard against SSR missing chrome
  drawer.setAttribute('aria-hidden', 'false');
  drawer.removeAttribute('inert'); // restore focus/interaction (attribute, not the
  // .inert property, which some in-app webviews don't reflect — see cart-drawer.js)
  openBtn.setAttribute('aria-expanded', 'true');
  document.body.classList.add('nav-open');
  drawer.addEventListener('keydown', trapFocus);
  closeBtn.focus();
}

export function closeDrawer() {
  const { drawer, openBtn } = getEls();
  if (!drawer || drawer.getAttribute('aria-hidden') === 'true') return;
  drawer.setAttribute('aria-hidden', 'true');
  drawer.setAttribute('inert', ''); // take descendants out of the tab order while hidden
  openBtn.setAttribute('aria-expanded', 'false');
  document.body.classList.remove('nav-open');
  drawer.removeEventListener('keydown', trapFocus);
  openBtn.focus();
}

export function initNavDrawer() {
  document.addEventListener('click', (e) => {
    if (e.target.closest('#nav-open')) openDrawer();
    else if (e.target.closest('#nav-close')) closeDrawer();
    else if (e.target.closest('#nav-overlay')) closeDrawer();
  });

  document.addEventListener('keydown', (e) => {
    const { drawer } = getEls();
    if (e.key === 'Escape' && drawer?.getAttribute('aria-hidden') === 'false') closeDrawer();
  });
}
