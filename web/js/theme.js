// theme.js — theme switching (dark/light, default dark)

const Theme = (() => {
  let state = localStorage.getItem('theme') || 'dark';

  function apply(mode) {
    if (mode === 'light') {
      document.documentElement.setAttribute('data-theme', 'light');
    } else {
      document.documentElement.removeAttribute('data-theme');
    }
    state = mode;
    localStorage.setItem('theme', mode);
    updateButton();
  }

  function toggle() {
    apply(state === 'dark' ? 'light' : 'dark');
  }

  function current() {
    return state;
  }

  function updateButton() {
    const btn = document.getElementById('theme-btn');
    if (btn) btn.textContent = state === 'dark' ? '🌙' : '☀';
  }

  return { apply, toggle, current, updateButton };
})();
