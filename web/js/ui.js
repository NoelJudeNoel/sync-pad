// ui.js — DOM operations, status, pairing screen, keyboard handling

const UI = (() => {
  const editor = () => document.getElementById('editor');
  const status = () => document.getElementById('status');
  const pairingScreen = () => document.getElementById('pairing-screen');
  const pairStatus = () => document.getElementById('pairStatus');
  const qrcodeContainer = () => document.getElementById('qrcode');

  let qrCode = null;
  let isPeerUpdate = false;
  let debounceTimer = null;
  let wakeLock = null;

  function setStatus(on) {
    const el = status();
    if (!el) return;
    el.textContent = on ? '●' : '○';
    el.style.color = on ? 'var(--status-on)' : 'var(--status-off)';
  }

  function setPairStatus(text) {
    const el = pairStatus();
    if (el) el.textContent = text;
  }

  function showPairing() {
    const screen = pairingScreen();
    if (screen) screen.classList.add('show');
  }

  function hidePairing() {
    const screen = pairingScreen();
    if (screen) screen.classList.remove('show');
  }

  function generateQR(roomId) {
    const container = qrcodeContainer();
    if (!container) return;
    container.innerHTML = '';
    const qrUrl = window.location.origin + '/s/' + roomId;
    qrCode = new QRCode(container, {
      text: qrUrl,
      width: 180,
      height: 180,
      colorDark: '#1a1a1a',
      colorLight: '#ffffff',
      correctLevel: QRCode.CorrectLevel.H
    });
  }

  function applyRemoteText(text) {
    const ed = editor();
    if (!ed || ed.value === text) return;

    const start = ed.selectionStart;
    const end = ed.selectionEnd;
    const wasFocused = document.activeElement === ed;

    isPeerUpdate = true;
    ed.value = text;

    if (wasFocused) {
      try { ed.setSelectionRange(Math.min(start, text.length), Math.min(end, text.length)); } catch(e) {}
    }
    isPeerUpdate = false;
  }

  function getEditorText() {
    const ed = editor();
    return ed ? ed.value : '';
  }

  function getEditorCursor() {
    const ed = editor();
    return ed ? ed.selectionStart : 0;
  }

  function isPeerUpdateActive() {
    return isPeerUpdate;
  }

  function sendIfDirty(callback) {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      callback();
    }, 200);
  }

  function focusEditor() {
    const ed = editor();
    if (ed) ed.focus();
  }

  function selectEditorAll() {
    const ed = editor();
    if (ed) ed.select();
  }

  function setCursorToEnd() {
    const ed = editor();
    if (ed) {
      const len = ed.value.length;
      try { ed.setSelectionRange(len, len); } catch(e) {}
    }
  }

  // Wake lock
  async function requestWakeLock() {
    try {
      if ('wakeLock' in navigator) {
        wakeLock = await navigator.wakeLock.request('screen');
        wakeLock.addEventListener('release', () => { wakeLock = null; });
      }
    } catch(e) {}
  }

  function releaseWakeLock() {
    if (wakeLock) { wakeLock.release(); wakeLock = null; }
  }

  // Keyboard detection
  function setupKeyboardHandler() {
    if (!window.visualViewport) return;

    window.visualViewport.addEventListener('resize', () => {
      const diff = window.innerHeight - window.visualViewport.height;
      const isKeyboard = diff > 80;
      document.body.classList.toggle('keyboard-open', isKeyboard);

      const bottomBar = document.getElementById('bottom-bar');
      if (bottomBar) {
        bottomBar.style.bottom = isKeyboard ? diff + 'px' : '';
      }
    });
  }

  function setupVisibilityHandler(onHidden) {
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') {
        releaseWakeLock();
        if (onHidden) onHidden();
      }
    });
  }

  return {
    setStatus, setPairStatus, showPairing, hidePairing, generateQR,
    applyRemoteText, getEditorText, getEditorCursor, isPeerUpdateActive,
    sendIfDirty, focusEditor, selectEditorAll, setCursorToEnd,
    requestWakeLock, releaseWakeLock,
    setupKeyboardHandler, setupVisibilityHandler
  };
})();
