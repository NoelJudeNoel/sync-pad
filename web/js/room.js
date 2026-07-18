// room.js — room ID generation and URL routing

const Room = (() => {
  function getPathRoom() {
    const parts = window.location.pathname.split('/');
    if (parts.length >= 3 && parts[1] === 's' && parts[2]) return parts[2];
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

  return { getPathRoom, generateRoomId };
})();
