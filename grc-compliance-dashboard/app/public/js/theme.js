// Theme toggle (dark/light)
const Theme = {
  init() {
    const saved = localStorage.getItem('ez-grc-theme') || 'light';
    this.set(saved);
  },

  set(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('ez-grc-theme', theme);
    const btn = document.getElementById('theme-toggle');
    if (btn) btn.textContent = theme === 'dark' ? '\u2600' : '\u263E';
  },

  toggle() {
    const current = document.documentElement.getAttribute('data-theme');
    this.set(current === 'dark' ? 'light' : 'dark');
  }
};
