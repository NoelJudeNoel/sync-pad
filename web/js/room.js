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
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    let id = '';
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
    for (let i = 0; i < 16; i++) id += chars[bytes[i] % chars.length];
    return id;
  }

  return { getPathRoom, generateRoomId, basePath };
})();
