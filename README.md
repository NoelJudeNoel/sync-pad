# 咕咕同步 — Sync Pad

跨设备实时同步文本输入。PC 打开页面生成二维码，手机扫码即连，两端输入实时同步。

手机靠近嘴边，小声使用任意输入法的语音输入功能，文字实时出现在 PC 上。

## 架构

```
PC 浏览器 ──WSS──→ Nginx (eu-as.cn:443) ──反代──→ Go sync-pad (:8080)
手机浏览器 ──WSS──→ Nginx (eu-as.cn:443) ──反代──→ Go sync-pad (:8080)
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
# 运行
make run

# 测试
make test

# 构建
make build
```

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

## 安全

| 机制 | 参数 |
|---|---|
| 速率限制 | 10 conn/min, 100 msg/min per IP |
| 消息大小上限 | 64KB |
| 连接上限 | 500/节点 |
| 房间 TTL | 30 分钟无活动自动清理 |
| 心跳 | 30s ping, 60s pong 超时 |
| Origin 校验 | 仅允许 eu-as.cn 域名 |

## 部署

### 环境要求

- Linux (systemd)
- Go 1.24+ (仅构建时需要)
- Nginx (反代)

### 构建与部署

```bash
make build    # 输出 ./sync-pad 单二进制
make deploy   # scp 到 abj 并重启服务
```

### systemd

```ini
[Unit]
Description=Sync Pad WebSocket Server
After=network.target

[Service]
Type=simple
ExecStart=/opt/sync-server/sync-pad
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

### Nginx

```nginx
location /s/ws {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_read_timeout 86400s;
}

location /s/ {
    alias /var/www/sync-text/;
    index index.html;
}
```

## 项目结构

```
sync-speech/
├── cmd/server/main.go              # 入口
├── internal/
│   ├── server/                     # HTTP + WS 服务
│   ├── room/                       # 房间管理
│   ├── ratelimit/                  # IP 级速率限制
│   └── config/                     # 配置常量
├── web/                            # 前端静态文件
│   ├── index.html
│   ├── css/                        # base/layout/components/themes
│   ├── js/                         # app/room/theme/websocket/ui
│   └── assets/
├── Makefile
└── README.md
```

## 访问

| 场景 | URL |
|---|---|
| 默认配对页 | `https://eu-as.cn/s/` |
| 指定房间 | `https://eu-as.cn/s/<roomId>` |

## 许可

MIT License
