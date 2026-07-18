# Sync Pad — 实时同步文本框

跨设备实时同步文本输入的极简工具。PC 生成二维码，手机扫码即连，两端输入实时同步。

---

## 架构

```
PC 浏览器 ──WSS──→ Nginx (eu-as.cn:443) ──反代──→ Node.js (:8080)
手机浏览器 ──WSS──→ Nginx (eu-as.cn:443) ──反代──→ Node.js (:8080)
                           ↓
                    内存态房间管理 (纯转发，不落盘)
```

---

## 文件结构

| 文件 | 位置 | 用途 |
|---|---|---|
| `server.js` | `/opt/sync-server/server.js` | WebSocket 服务端，房间管理，心跳 |
| `package.json` | `/opt/sync-server/package.json` | Node.js 依赖 (`ws`) |
| `index.html` | `/var/www/sync-text/index.html` | 单页前端，含编辑器/配对/二维码/主题切换 |
| Nginx 配置 | `/etc/nginx/sites-enabled/eu-as.cn` | 反代 `/s/` 静态 + `/s/ws` WebSocket |

---

## WebSocket 协议

端点: `wss://eu-as.cn/s/ws?room=<roomId>`

### 消息格式 (JSON)

| 方向 | 字段 | 说明 |
|---|---|---|
| 服务端 → 新连接 | `{"d":"<文本>"}` | 推送房间当前文本 |
| 服务端 → 已有客户端 | `{"t":"j"}` | 有人加入房间 |
| 服务端 → 剩余客户端 | `{"t":"l","c":<剩余人数>}` | 有人离开房间 |
| 客户端 → 服务端 | `{"d":"<文本>","p":<光标位置>}` | 本地输入更新 |
| 服务端 → 其他客户端 | `{"d":"<文本>","p":<光标位置>}` | 广播远端输入 |

### 生命期事件

```
PC 连接:        → (服务端推送当前文本) → 显示编辑器
手机加入:       → PC 收到 {"t":"j"}    → 建立配对
手机输入:       → PC 收到 {"d":"...","p":N} → 更新编辑器+光标
手机断开:       → PC 收到 {"t":"l","c":1} → 弹出二维码覆盖层
手机重连:       → PC 收到 {"t":"j"}    → 切回编辑模式
```

---

## 服务端 (server.js)

**启动**: `pm2 start /opt/sync-server/server.js --name sync-pad`

### 关键机制

| 机制 | 实现 |
|---|---|
| 心跳 | 每 2s ping 所有客户端，无 pong 则 `terminate()` |
| 房间管理 | `Map<roomId, {clients: Set<WebSocket>, text: string}>` |
| 文本持久 | 每房间存最新文本，新连接自动推送 |
| 加入通知 | 非首位连接者 → 广播 `{"t":"j"}` 给已有客户端 |
| 离开通知 | 房间非空时 → 广播 `{"t":"l","c":N}` 给剩余客户端 |
| 房间 ID 校验 | 正则 `/^[a-zA-Z0-9_-]{8,128}$/`，不合法则分配随机 hex |

---

## 前端 (index.html)

单 HTML 文件，无框架依赖。外部 CDN 引用: `qrcodejs` (二维码生成)。

### 页面状态机

```
                        ┌─────────────┐
                        │  配对页      │  (初始)
                        │  显示二维码  │
                        └──────┬──────┘
                               │ 手机扫码加入 ({"t":"j"})
                               ↓
                        ┌─────────────┐
                        │  编辑模式    │  ← 两端输入实时同步
                        │  paired=true │
                        └──────┬──────┘
                               │ 手机断线 ({"t":"l","c":1})
                               ↓
                        ┌─────────────┐
                        │  重配覆盖层  │  (保留编辑器内容)
                        │  显示新二维码│
                        └──────┬──────┘
                               │ 手机重连 ({"t":"j"})
                               ↓
                        ┌─────────────┐
                        │  编辑模式    │
                        └─────────────┘
```

### 状态变量

| 变量 | 含义 |
|---|---|
| `connected` | WebSocket 连接状态（与服务器） |
| `paired` | 是否已配对（≥2 人同房间） |
| `isPairingMode` | 初始配对模式（PC 刚打开页面） |
| `isRePairing` | 断线后重配模式 |
| `isPeerUpdate` | 是否正在应用远端更新（防回响） |

### 底部栏元素

从左到右: `剪切` / `复制` / `主题切换(三态)` / `关于本站` / `备案号` / `公网安备号`

- 键盘弹出时：备案号隐藏，底部栏紧贴键盘上方 (`visualViewport` API)
- 主题三态: system → dark → light → system

### 其他特性

| 特性 | 实现 |
|---|---|
| 唤醒锁 | `navigator.wakeLock.request('screen')`，防止手机息屏 |
| 光标同步 | 发送端附带 `p` 字段，接收端 `setSelectionRange` |
| 断连重连 | 3s 后自动重连 WebSocket |
| 安全房间号 | `crypto.getRandomValues()` 生成 16 字节随机串 |
| 二维码重配 | 手机断线后覆盖层展示新二维码，保留编辑器文本 |

---

## 部署 (abj 主机)

**SSH**: `ssh abj` (39.106.35.187, root)

| 路径 | 说明 |
|---|---|
| `/opt/sync-server/server.js` | 服务端代码 |
| `/opt/sync-server/node_modules/` | npm 依赖 |
| `/var/www/sync-text/index.html` | 前端页面 |
| `/etc/nginx/sites-enabled/eu-as.cn` | Nginx 配置 |

### Nginx 关键配置

```nginx
# WebSocket 反代
location /s/ws {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_read_timeout 86400s;
}

# 静态文件 + SPA 回退
location /s/ {
    alias /var/www/sync-text/;
    index index.html;
    try_files $uri /s/index.html;
}
```

### 运维命令

```bash
# 重启服务
pm2 restart sync-pad

# 查看日志
pm2 logs sync-pad

# 部署前端（本机 → abj）
scp index.html abj:/var/www/sync-text/

# 部署后端（本机 → abj）
scp server.js abj:/opt/sync-server/
ssh abj "pm2 restart sync-pad"

# 重载 Nginx
ssh abj "nginx -t && nginx -s reload"
```

---

## 访问

| 场景 | URL |
|---|---|
| 默认配对页 | `https://eu-as.cn/s/` |
| 指定房间 | `https://eu-as.cn/s/<roomId>` |

---

## 可能的扩展方向

- **历史版本**: 服务端定时落盘 MySQL/PostgreSQL
- **PWA**: 添加 manifest.json，手机可添加到桌面
- **同步光标**: 当前是全量文本+光标位置，可引入 OT 算法
- **多端冲突**: 目前最后写入覆盖，概率低可接受
- **WebRTC**: 点对点传输降低服务端负载