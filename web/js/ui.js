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
  let pendingRemote = null;   // {text, remotePos} queued while a local selection is active
  let pendingRemoteTimer = null;

  // If the user leaves a selection sitting untouched, don't withhold
  // remote updates forever — apply after this cap even without a
  // selectionchange/mouseup signal, so the pad never goes silently stale.
  const PENDING_REMOTE_MAX_DELAY_MS = 8000;

  // Heuristic warning threshold in JS string length (chars, not bytes).
  // Backend default is SYNC_PAD_MAX_MESSAGE_SIZE = 64KB on the raw JSON
  // frame; multi-byte (e.g. CJK) text can exceed that well before 64000
  // characters, so this is a conservative early warning, not a hard limit
  // enforced client-side. The server is still the source of truth and will
  // reject/log oversized frames on its own.
  const LENGTH_WARN_THRESHOLD = 30000;

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
    const base = Room.basePath();
    const qrUrl = window.location.origin + base + '/' + roomId;
    qrCode = new QRCode(container, {
      text: qrUrl,
      width: 180,
      height: 180,
      colorDark: '#1a1a1a',
      colorLight: '#ffffff',
      correctLevel: QRCode.CorrectLevel.H
    });
  }

  function doApplyRemoteText(text, remotePos) {
    const ed = editor();
    if (!ed) return;

    isPeerUpdate = true;
    ed.value = text;

    if (remotePos !== undefined && typeof remotePos === 'number') {
      const pos = Math.min(remotePos, text.length);
      try { ed.setSelectionRange(pos, pos); } catch(e) {}
    }
    isPeerUpdate = false;
  }

  function flushPendingRemote() {
    if (!pendingRemote) return;
    clearTimeout(pendingRemoteTimer);
    pendingRemoteTimer = null;
    const { text, remotePos } = pendingRemote;
    pendingRemote = null;
    doApplyRemoteText(text, remotePos);
  }

  // Remote updates normally overwrite the textarea outright. But if the
  // user currently has an active (non-collapsed) selection — the exact
  // moment right before a manual copy — overwriting would silently drop
  // their selection and relocate the caret. Instead, queue the latest
  // incoming update and apply it once the selection collapses (selection
  // cleared, or the user clicks/types elsewhere), capped at
  // PENDING_REMOTE_MAX_DELAY_MS so the pad can't go stale indefinitely.
  function applyRemoteText(text, remotePos) {
    const ed = editor();
    if (!ed) return;

    if (ed.selectionStart !== ed.selectionEnd) {
      pendingRemote = { text, remotePos };
      if (!pendingRemoteTimer) {
        pendingRemoteTimer = setTimeout(flushPendingRemote, PENDING_REMOTE_MAX_DELAY_MS);
      }
      return;
    }

    doApplyRemoteText(text, remotePos);
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

  function checkTextLength(text) {
    const el = status();
    if (!el) return;
    if (text.length > LENGTH_WARN_THRESHOLD) {
      el.classList.add('status-warn');
    } else {
      el.classList.remove('status-warn');
    }
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

  // Selection may collapse via mouse, keyboard, or focus loss — 'selectionchange'
  // covers all of these, so it's the single hook needed to release a
  // deferred remote update once the user is done selecting.
  document.addEventListener('selectionchange', () => {
    if (!pendingRemote) return;
    const ed = editor();
    if (ed && document.activeElement === ed && ed.selectionStart !== ed.selectionEnd) {
      return; // still actively selecting, keep waiting
    }
    flushPendingRemote();
  });

  return {
    setStatus, setPairStatus, showPairing, hidePairing, generateQR,
    applyRemoteText, getEditorText, getEditorCursor, isPeerUpdateActive,
    sendIfDirty, checkTextLength, focusEditor, selectEditorAll, setCursorToEnd,
    requestWakeLock, releaseWakeLock,
    setupKeyboardHandler, setupVisibilityHandler
  };
})();
