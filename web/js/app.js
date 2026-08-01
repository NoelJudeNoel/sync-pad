// app.js — entry point + state machine

(() => {
  // State
  let connected = false;
  let paired = false;
  let isPairingMode = false;
  let isRePairing = false;
  let roomId = '';
  let qrGenerated = false;

  // Initialize
  function init() {
    roomId = Room.getPathRoom();

    if (!roomId) {
      isPairingMode = true;
      roomId = Room.generateRoomId();
      document.body.classList.remove('editor-mode');
      UI.generateQR(roomId);
    } else {
      document.body.classList.add('editor-mode');
      UI.hidePairing();
    }

    Theme.apply(Theme.current());
    UI.setupKeyboardHandler();
    UI.setupVisibilityHandler();
    setupClipboardButtons();

    WS.setHandlers({
      onMessage: handleMessage,
      onStatusChange: handleStatusChange
    });

    WS.connect(roomId);
    UI.setStatus(false);
  }

  function handleStatusChange(on) {
    connected = on;
    UI.setStatus(on);

    if (isRePairing) {
      UI.setPairStatus(on ? '✓ 服务已连接，等待手机重新扫码...' : '服务断开，重连中...');
    } else if (isPairingMode) {
      UI.setPairStatus(on ? '✓ 已连接，手机扫码加入' : '连接断开，重连中...');
    }

    if (on) {
      UI.requestWakeLock();
    } else {
      UI.releaseWakeLock();
    }
  }

  function handleMessage(raw) {
    let msg;
    try {
      msg = JSON.parse(raw);
    } catch(e) {
      return;
    }

    // Peer joined
    if (msg.t === 'j') {
      if (isPairingMode || isRePairing) {
        isPairingMode = false;
        isRePairing = false;
        paired = true;
        document.body.classList.add('editor-mode');
        UI.hidePairing();
        UI.focusEditor();
        history.pushState(null, '', Room.basePath() + '/' + roomId);
      }
      return;
    }

    // Peer left — show re-pairing if alone
    if (msg.t === 'l' && msg.c === 1 && paired) {
      showRePairingScreen();
      return;
    }

    // Text update
    if (msg.d !== undefined) {
      UI.applyRemoteText(msg.d, msg.p);
    }
  }

  function showRePairingScreen() {
    isRePairing = true;
    paired = false;
    UI.showPairing();
    UI.setPairStatus('手机已断开，请重新扫码');
    UI.generateQR(roomId);
    UI.releaseWakeLock();
  }

  // Input handling
  function setupInputHandler() {
    const ed = document.getElementById('editor');
    if (!ed) return;

    ed.addEventListener('input', () => {
      if (UI.isPeerUpdateActive()) return;
      UI.checkTextLength(UI.getEditorText());
      UI.sendIfDirty(() => {
        WS.send(JSON.stringify({
          p: UI.getEditorCursor(),
          d: UI.getEditorText()
        }));
      });
    });
  }

  // Clipboard buttons
  function setupClipboardButtons() {
    const cutBtn = document.getElementById('cut-btn');
    const copyBtn = document.getElementById('copy-btn');

    if (cutBtn) {
      cutBtn.addEventListener('click', () => {
        UI.focusEditor();
        const ed = document.getElementById('editor');
        if (ed.selectionStart !== ed.selectionEnd) {
          document.execCommand('cut');
        } else {
          ed.select();
          document.execCommand('cut');
        }
      });
    }

    if (copyBtn) {
      copyBtn.addEventListener('click', () => {
        UI.focusEditor();
        const ed = document.getElementById('editor');
        if (ed.selectionStart !== ed.selectionEnd) {
          document.execCommand('copy');
        } else {
          ed.select();
          document.execCommand('copy');
        }
        setTimeout(() => UI.setCursorToEnd(), 100);
      });
    }
  }

  // Theme button
  function setupThemeButton() {
    const btn = document.getElementById('theme-btn');
    if (btn) {
      btn.addEventListener('click', () => Theme.toggle());
    }
  }

  // Start
  setupInputHandler();
  setupThemeButton();
  init();
})();
