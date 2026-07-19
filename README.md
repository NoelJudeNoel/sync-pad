# Sync Pad — 咕咕同步

跨设备实时文本同步。PC 打开页面生成二维码，手机扫码即连，两端输入实时同步。

手机靠近嘴边，小声使用任意输入法的语音输入，文字实时出现在 PC 上。

## 架构

```
PC 浏览器 ──WSS──→ Nginx (:443) ──反代──→ Go sync-pad (:8080)
手机浏览器 ──WSS──→ Nginx (:443) ──反代──→ Go sync-pad (:8080)
                            ↓
                     内存态房间管理 (纯转发，不落盘)
```

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.24+ (gorilla/websocket) |
| 前端 | 纯 HTML/CSS/JS，零构建步骤 |
| 部署 | 单二进制 + systemd + Nginx |

## 快速开始

```bash
# 运行 (默认监听 :8080，web 目录 ./web)
make run

# 测试
make test

# 构建
make build    # 输出 ./sync-pad 单二进制
```

打开 `http://localhost:8080/` 即可使用。

## 配置

配置通过环境变量传入，全部以 `SYNC_PAD_` 前缀开头。未设置时使用默认值：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `SYNC_PAD_PORT` | `:8080` | 监听地址 |
| `SYNC_PAD_WEB_DIR` | `./web` | 静态文件 (前端) 目录 |
| `SYNC_PAD_BASE_PATH` | `/s` | HTTP 路由前缀 (如 `/sync`) |
| `SYNC_PAD_MAX_CONNECTIONS` | `500` | 最大并发连接数 |
| `SYNC_PAD_ROOM_TTL` | `30m` | 无活动房间存活时间 |
| `SYNC_PAD_PING_PERIOD` | `3s` | WebSocket ping 间隔 |
| `SYNC_PAD_PONG_WAIT` | `6s` | 等待 pong 超时 |
| `SYNC_PAD_RATE_LIMIT_CONNS` | `10` | 每 IP 每分钟最大连接请求 |
| `SYNC_PAD_RATE_LIMIT_MESSAGES` | `100` | 每 IP 每分钟最大消息数 |
| `SYNC_PAD_ALLOWED_ORIGINS` | `http://localhost,...` | 逗号分隔的允许 Origin 列表 |

例：

```bash
SYNC_PAD_PORT=:9090 \
SYNC_PAD_BASE_PATH=/sync \
SYNC_PAD_ALLOWED_ORIGINS="https://your-domain.com" \
./sync-pad
```

## WebSocket 协议

端点: `wss://your-domain.com<base-path>/ws?room=<roomId>`

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
PC 连接        → 服务端推送当前文本 → 显示编辑器
手机加入       → PC 收到 {"t":"j"}  → 建立配对，隐藏二维码
手机输入       → PC 收到 {"d":"...","p":N} → 更新编辑器 + 光标
手机断开       → PC 收到 {"t":"l","c":1} → 弹出二维码等待重连
手机重连       → PC 收到 {"t":"j"}  → 切回编辑模式
```

## 安全

| 机制 | 默认值 |
|---|---|
| 速率限制 | 10 conn/min, 100 msg/min per IP |
| 消息大小上限 | 64KB |
| 连接上限 | 500/节点 |
| 房间 TTL | 30 分钟无活动自动清理 |
| 心跳 | 3s ping, 6s pong |
| Origin 校验 | 限定 `SYNC_PAD_ALLOWED_ORIGINS` |

## 部署

### 本地 (开发)

```bash
make run
# 浏览器打开 http://localhost:8080/
```

### 生产 (systemd + Nginx)

1. 构建二进制：

   ```bash
   make build    # 输出 ./sync-pad
   ```

2. 复制到服务器：

   ```bash
   scp sync-pad your-host:/usr/local/bin/
   scp -r web your-host:/var/www/sync-pad/
   ```

3. 使用 systemd (`deploy/sync-pad.service` 是模板)：

   ```ini
   [Unit]
   Description=Sync Pad WebSocket Server
   After=network.target

   [Service]
   Type=simple
   ExecStart=/usr/local/bin/sync-pad
   Restart=always
   RestartSec=3
   EnvironmentFile=/etc/sync-pad/env

   [Install]
   WantedBy=multi-user.target
   ```

   通过 `EnvironmentFile` 注入环境变量 (`deploy/sync-pad.env.example` 是样本)。

   ```bash
   # 启用并启动服务
   sudo systemctl enable --now sync-pad
   ```

4. Nginx 反代：

   ```nginx
   # WebSocket
   location /s/ws {
       proxy_pass http://127.0.0.1:8080;
       proxy_http_version 1.1;
       proxy_set_header Upgrade $http_upgrade;
       proxy_set_header Connection "upgrade";
       proxy_set_header Host $host;
       proxy_read_timeout 86400s;
   }

   # 静态资源
   location /s/ {
       alias /var/www/sync-pad/;
       index index.html;
   }
   ```

   将 `/s/ws` 和 `/s/` 换成你实际配置的 `SYNC_PAD_BASE_PATH` 值。

5. 前端路径同步：

   如果修改了 `SYNC_PAD_BASE_PATH` (默认 `/s`)，需同步修改：
   - `web/index.html` 中的 `<meta name="base-path" content="/s">`
   - `web/index.html` 中静态资源的引用路径 (如 `/s/css/...`、`/s/js/...`、`/s/assets/...`)
   - `web/manifest.webmanifest` 中的 `start_url`、`scope` 和图标路径

## 项目结构

```
sync-pad/
├── cmd/server/main.go              # 入口
├── internal/
│   ├── server/                     # HTTP + WS 服务
│   ├── room/                       # 房间管理
│   ├── ratelimit/                  # IP 级速率限制
│   └── config/                     # 配置加载
├── web/                            # 前端静态文件
│   ├── index.html
│   ├── css/                        # base/layout/components/themes
│   ├── js/                         # app/room/theme/websocket/ui
│   ├── assets/                     # logo、图标
│   └── manifest.webmanifest
├── deploy/
│   ├── sync-pad.service            # systemd 单元模板
│   └── sync-pad.env.example        # 环境变量样本
├── Makefile
├── Dockerfile
├── CONTRIBUTING.md
├── LICENSE
└── README.md
```

## 许可

MIT License
