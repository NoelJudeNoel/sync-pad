const WebSocket = require('ws');
const http = require('http');
const crypto = require('crypto');

const server = http.createServer();
const wss = new WebSocket.Server({ server });

const rooms = new Map(); // Map<roomId, { clients: Set<WebSocket>, text: string }>

// ─── 心跳检测 ───
const HEARTBEAT_INTERVAL = 2000;
const hbTimer = setInterval(() => {
  wss.clients.forEach((ws) => {
    if (ws._alive === false) { ws.terminate(); return; }
    ws._alive = false;
    ws.ping();
  });
}, HEARTBEAT_INTERVAL);
wss.on('close', () => clearInterval(hbTimer));

// ─── 房间 ID 校验 ───
function validRoomId(id) {
  return typeof id === 'string' && /^[a-zA-Z0-9_-]{8,128}$/.test(id);
}

wss.on('connection', (ws, req) => {
  ws._alive = true;
  ws.on('pong', () => { ws._alive = true; });

  const url = new URL(req.url, `http://${req.headers.host}`);
  const rawRoom = url.searchParams.get('room') || '';
  const roomId = validRoomId(rawRoom) ? rawRoom
    : crypto.randomBytes(8).toString('hex');

  if (!rooms.has(roomId)) {
    rooms.set(roomId, { clients: new Set(), text: '' });
  }
  const room = rooms.get(roomId);
  room.clients.add(ws);

  // 非首位用户通知已有客户端
  if (room.clients.size > 1) {
    room.clients.forEach((client) => {
      if (client !== ws && client.readyState === WebSocket.OPEN) {
        client.send(JSON.stringify({ t: 'j' }));
      }
    });
  }

  console.log(`[Room: ${roomId}] 新连接，当前 ${room.clients.size} 人`);

  // 推送当前房间文本
  ws.send(JSON.stringify({ d: room.text }));

  ws.on('message', (data) => {
    try {
      const msg = JSON.parse(data.toString());
      room.text = msg.d || '';
      room.clients.forEach((client) => {
        if (client !== ws && client.readyState === WebSocket.OPEN) {
          client.send(JSON.stringify({ d: msg.d, p: msg.p }));
        }
      });
    } catch (e) {
      console.error(`[Room: ${roomId}] 解析失败: ${e.message}`);
    }
  });

  ws.on('close', () => {
    room.clients.delete(ws);
    if (room.clients.size === 0) {
      rooms.delete(roomId);
      console.log(`[Room: ${roomId}] 房间已清空`);
    } else {
      // 通知剩余客户端有人离开
      room.clients.forEach((client) => {
        if (client.readyState === WebSocket.OPEN) {
          client.send(JSON.stringify({ t: 'l', c: room.clients.size }));
        }
      });
      console.log(`[Room: ${roomId}] 断开，剩余 ${room.clients.size} 人`);
    }
  });
});

server.listen(8080, () => {
  console.log('SyncPad 服务启动，端口 8080');
});