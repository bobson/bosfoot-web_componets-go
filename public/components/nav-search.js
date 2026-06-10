function getEls() {
  return {
    dropdown: document.getElementById('search-dropdown'),
    btn:      document.getElementById('search-btn'),
    input:    document.getElementById('search-input'),
  }
}

export function openSearch() {
  const { dropdown, btn, input } = getEls()
  dropdown.setAttribute('aria-hidden', 'false')
  btn.setAttribute('aria-expanded', 'true')
  input?.focus()
}

export function closeSearch() {
  const { dropdown, btn } = getEls()
  if (dropdown.getAttribute('aria-hidden') === 'true') return
  dropdown.setAttribute('aria-hidden', 'true')
  btn.setAttribute('aria-expanded', 'false')
  btn?.focus()
}

document.addEventListener('click', e => {
  if (e.target.closest('#search-btn')) {
    openSearch()
  } else if (e.target.closest('#search-close')) {
    closeSearch()
  } else if (!e.target.closest('#search-dropdown')) {
    closeSearch()
  }
})

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') closeSearch()
})
