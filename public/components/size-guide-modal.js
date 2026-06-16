export function initSizeGuideModal() {
  const openBtn = document.getElementById('open-size-guide');
  const modal = document.getElementById('size-guide-modal');
  const closeBtn = modal?.querySelector('.sg-modal__close');

  if (!modal || !openBtn) return;

  // Open modal
  openBtn.addEventListener('click', () => {
    modal.showModal();
    document.body.style.overflow = 'hidden';
  });

  // Close modal
  const closeModal = () => {
    modal.close();
    document.body.style.overflow = '';
  };

  closeBtn?.addEventListener('click', closeModal);

  // Close on backdrop click (click outside the dialog)
  modal.addEventListener('click', (e) => {
    if (e.target === modal) {
      closeModal();
    }
  });

  // Close on Escape key
  modal.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      closeModal();
    }
  });
}
