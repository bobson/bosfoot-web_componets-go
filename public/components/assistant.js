// FAQ assistant widget. Static chrome is server-rendered in the "assistant"
// partial; this only adds behaviour.
//   - Quick questions are a native <details> accordion (answer below the
//     question) — they work with no JS and no network.
//   - The free-text box (present only when the server enabled it) POSTs to
//     /api/assistant and appends the reply as an accordion item in the same
//     look, so a typed question also shows its answer directly below it.

const CHEVRON =
  '<svg class="assistant__chevron" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="6 9 12 15 18 9"/></svg>';

function getEls() {
  return {
    root: document.getElementById('assistant'),
    launcher: document.getElementById('assistant-launcher'),
    panel: document.getElementById('assistant-panel'),
    close: document.getElementById('assistant-close'),
    log: document.getElementById('assistant-log'),
    form: document.getElementById('assistant-form'),
    input: document.getElementById('assistant-input'),
  };
}

function openPanel() {
  const { panel, launcher, input } = getEls();
  if (!panel) return;
  panel.setAttribute('aria-hidden', 'false');
  panel.removeAttribute('inert');
  launcher?.setAttribute('aria-expanded', 'true');
  input?.focus();
}

function closePanel() {
  const { panel, launcher } = getEls();
  if (!panel || panel.getAttribute('aria-hidden') === 'true') return;
  panel.setAttribute('aria-hidden', 'true');
  panel.setAttribute('inert', '');
  launcher?.setAttribute('aria-expanded', 'false');
  launcher?.focus();
}

// Append an open accordion item for a typed question and return its answer <p>,
// so the caller can swap the placeholder for the real answer. Question and
// answer are set via textContent, so any text stays inert.
function addReply(question, answerText) {
  const { log } = getEls();
  if (!log) return null;

  const item = document.createElement('details');
  item.className = 'assistant__item assistant__item--reply';
  item.open = true;

  const summary = document.createElement('summary');
  summary.className = 'assistant__q';
  const qSpan = document.createElement('span');
  qSpan.textContent = question;
  summary.appendChild(qSpan);
  summary.insertAdjacentHTML('beforeend', CHEVRON);

  const answer = document.createElement('div');
  answer.className = 'assistant__a';
  const p = document.createElement('p');
  p.textContent = answerText;
  answer.appendChild(p);

  item.append(summary, answer);
  log.appendChild(item);
  log.scrollTop = log.scrollHeight;
  return p;
}

async function askServer() {
  const { root, input, form } = getEls();
  if (!root || !input) return;
  const question = input.value.trim();
  if (!question) return;

  input.value = '';
  const answerP = addReply(question, root.dataset.msgThinking || '…');
  answerP?.parentElement.classList.add('assistant__a--pending');
  const send = form?.querySelector('.assistant__send');
  if (send) send.disabled = true;

  try {
    const res = await fetch(root.dataset.endpoint || '/api/assistant', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question, locale: root.dataset.locale || 'mk' }),
    });
    if (!res.ok) throw new Error(`status ${res.status}`);
    const data = await res.json();
    if (answerP) answerP.textContent = (data.answer || '').trim() || root.dataset.msgError || '';
  } catch {
    if (answerP) answerP.textContent = root.dataset.msgError || 'Error';
  } finally {
    answerP?.parentElement.classList.remove('assistant__a--pending');
    if (send) send.disabled = false;
    input.focus();
  }
}

export function initAssistant() {
  const { root, launcher, close, form } = getEls();
  if (!root || !launcher) return;

  launcher.addEventListener('click', () => {
    if (launcher.getAttribute('aria-expanded') === 'true') closePanel();
    else openPanel();
  });
  close?.addEventListener('click', closePanel);

  form?.addEventListener('submit', (e) => {
    e.preventDefault();
    askServer();
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closePanel();
  });
}
