// room.js — room ID generation and URL routing

const Room = (() => {
  function basePath() {
    const meta = document.querySelector('meta[name="base-path"]');
    if (meta && meta.content) {
      return meta.content.replace(/\/+$/, '');
    }
    return '';
  }

  function getPathRoom() {
    const base = basePath();
    const parts = window.location.pathname.replace(/^\//, '').split('/');
    const baseParts = base.replace(/^\//, '').split('/');

    // Strip base prefix, next segment is room ID (if present)
    let i = 0;
    while (i < baseParts.length && i < parts.length && parts[i] === baseParts[i]) {
      i++;
    }

    if (i === baseParts.length && parts[i]) {
      return parts[i];
    }
    return '';
  }

  function generateRoomId() {
    // 32-character alphabet (a-z + 2-7, RFC4648 base32 lowercase) so that
    // 256 % 32 === 0 — masking with & 31 maps each random byte onto the
    // alphabet with zero bias. The previous 37-char alphabet ('a-z0-9')
    // didn't divide 256 evenly, over-representing the first 34 characters.
    // Mirrors the server's fallback generator in websocket.go.
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    const chars = 'abcdefghijklmnopqrstuvwxyz234567';
    let id = '';
    for (let i = 0; i < 16; i++) id += chars[bytes[i] & 31];
    return id;
  }

  return { getPathRoom, generateRoomId, basePath };
})();
