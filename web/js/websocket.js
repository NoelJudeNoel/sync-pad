// websocket.js — connection, reconnect, message protocol

const WS = (() => {
  let ws = null;
  let reconnectTimer = null;
  let reconnectDelay = 3000;
  let onMessage = null;
  let onStatusChange = null;

  function connect(roomId) {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const base = Room.basePath();
    const url = `${proto}://${window.location.host}${base}/ws?room=${roomId}`;
    ws = new WebSocket(url);

    ws.onopen = () => {
      reconnectDelay = 3000;
      if (onStatusChange) onStatusChange(true);
    };

    ws.onmessage = (event) => {
      if (onMessage) onMessage(event.data);
    };

    ws.onclose = () => {
      if (onStatusChange) onStatusChange(false);
      clearTimeout(reconnectTimer);
      reconnectTimer = setTimeout(() => connect(roomId), reconnectDelay);
      reconnectDelay = Math.min(reconnectDelay * 1.5, 30000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }

  function send(data) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data);
    }
  }

  function setHandlers({ onMessage: msg, onStatusChange: status }) {
    onMessage = msg;
    onStatusChange = status;
  }

  return { connect, send, setHandlers };
})();
