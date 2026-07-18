# Sync Pad 重设计规格说明书

**日期**: 2026-07-18
**状态**: 设计已完成，待实施
**规模**: ~1K DAU, ~100 CCU（小规模）

---

## 1. 项目定位

**咕咕同步** — 手机靠近嘴边，小声使用任意输入法的语音输入功能，文字实时同步到 PC 端。

核心定位是**纯同步通道**，不做 ASR，不做通用协作编辑器。手机是输入端，PC 是接收端。

项目将完全开源，作为引流重点项目对外开放。

---

## 2. 范围与非范围

### In Scope
- WebSocket 实时文本同步
- 手机端键盘升起时的底部栏交互修复
- 安全加固（速率限制、消息大小限制、连接上限、房间 TTL）
- Go 后端重写（替代 Node.js）
- 前端 UI 重设计（分离 HTML/CSS/JS，移动优先）
- 开源准备（LICENSE, CONTRIBUTING, 工程化）

### Out of Scope
- ASR / 语音识别
- 富文本 / 多人协作 / OT 算法
- 数据持久化 / 历史版本
- 用户系统 / 认证
- PWA / App
- WebRTC 点对点

---

## 3. 设计决策

### 3.1 后端语言: Go

**决策**: 用 Go + gorilla/websocket 重写服务端
**理由**: 单二进制部署、零运行时依赖、开源部署极简单、小规模下性能远超需求
**权衡**: 放弃了当前 Node.js 生态（需要重写），但换来单文件部署 + 更低资源占用

### 3.2 无外部依赖

**决策**: 不依赖 Redis、PostgreSQL、任何云服务
**理由**: 小规模下内存态 + 单进程完全够用，运维极简是开源项目的竞争力
**权衡**: 无法多节点水平扩展，但 ~1K DAU 不需要

### 3.3 前端分离

**决策**: HTML / CSS / JS 三文件分离，不使用框架
**理由**: 保持零构建步骤（开源项目 clone 即跑），同时获得可维护性
**权衡**: 没有 React/Vue 的组件化，但项目规模小，不值得引入构建链

### 3.4 主题简化

**决策**: 从三态（system → dark → light）简化为二态（dark ↔ light），首次进入跟随系统
**理由**: 三态循环 UI 复杂且非必要
**权衡**: 用户不能主动切回"跟随系统"模式，但可通过清除 storage 重置

---

## 4. 架构

### 4.1 拓扑

```
PC 浏览器 (接收端)             手机浏览器 (输入端)
https://eu-as.cn/s/xxx        扫描 QR 后进入同房间
        │                            │
        └──────── WSS ───────────────┘
                     │
            ┌────────▼────────┐
            │  Nginx (443)    │
            │  反代 /s/ws     │
            │  静态资源 /s/   │
            └────────┬────────┘
                     │
            ┌────────▼────────┐
            │  Go sync-pad    │
            │  :8080          │
            │                 │
            │  ┌───────────┐  │
            │  │ Room Mgr  │  │
            │  │ (sync.Map)│  │
            │  └───────────┘  │
            │  ┌───────────┐  │
            │  │ Rate      │  │
            │  │ Limiter   │  │
            │  └───────────┘  │
            │  ┌───────────┐  │
            │  │ Healthz   │  │
            │  └───────────┘  │
            └─────────────────┘
```

### 4.2 项目结构

```
sync-speech/
├── cmd/
│   └── server/
│       └── main.go                  # 入口
├── internal/
│   ├── server/
│   │   ├── server.go                # HTTP 服务启动
│   │   ├── websocket.go             # WS 升级、读写循环
│   │   └── middleware.go            # recover、日志、Origin 校验
│   ├── room/
│   │   ├── manager.go               # 房间 CRUD + TTL 清理
│   │   └── room.go              # Room/Client 数据结构
│   ├── ratelimit/
│   │   └── limiter.go               # per-IP 令牌桶
│   └── config/
│       └── config.go            # 常量配置
├── web/
│   ├── index.html                   # 纯结构
│   ├── css/
│   │   ├── base.css             # CSS 变量 + reset
│   │   ├── layout.css           # 编辑器 / 底部栏 / 覆盖层
│   │   ├── components.css       # 按钮 / 状态点 / QR 容器
│   │   └── themes.css           # 暗色 / 亮色
│   ├── js/
│   │   ├── app.js               # 入口 + 状态机
│   │   ├── websocket.js         # WS 连接 / 重连 / 协议
│   │   ├── room.js              # 房间 ID 生成 / URL 路由
│   │   ├── ui.js                # DOM 操作 / 底部栏
│   │   └── theme.js             # 主题切换
│   └── assets/
│       └── icon-192.png         # 简易图标
├── go.mod
├── Makefile
├── Dockerfile
├── LICENSE
├── CONTRIBUTING.md
└── README.md
```

---

## 5. 后端设计（Go）

### 5.1 依赖

| 库 | 用途 |
|---|---|
| `github.com/gorilla/websocket` | WebSocket 协议 |
| 标准库 `net/http` | HTTP 服务 |
| 标准库 `log/slog` | 结构化日志 |
| 标准库 `sync` | 并发安全 |

### 5.2 数据结构

```go
// Client 代表一个 WebSocket 连接
type Client struct {
    conn   *websocket.Conn
    room   *Room
    send   chan []byte
}

// Room 代表一个同步房间
type Room struct {
    id      string
    clients map[*Client]struct{}
    text    string
    mu      sync.RWMutex
    expiry  time.Time
}

// Manager 管理所有房间
type Manager struct {
    rooms map[string]*Room
    mu    sync.RWMutex
}
```

### 5.3 WebSocket 协议（向后兼容）

端点: `wss://eu-as.cn/s/ws?room=<roomId>`

| 方向 | JSON | 说明 |
|---|---|---|
| 服务端 → 新连接 | `{"d":"<文本>"}` | 推送房间当前文本 |
| 服务端 → 已有客户端 | `{"t":"j"}` | 有人加入房间 |
| 服务端 → 剩余客户端 | `{"t":"l","c":<剩余人数>}` | 有人离开房间 |
| 客户端 → 服务端 | `{"d":"<文本>","p":<光标位置>}` | 本地输入更新 |
| 服务端 → 其他客户端 | `{"d":"<文本>","p":<光标位置>}` | 广播远端输入 |

### 5.4 安全参数

| 参数 | 值 | 说明 |
|---|---|---|
| MaxMessageSize | 64KB | 单帧上限 |
| MaxConnections | 500 | 单节点最大连接 |
| RateLimit conn | 10/min | IP 级连接限制 |
| RateLimit message | 100/min | IP 级消息限制 |
| RoomTTL | 30min | 无活动自动清理 |
| WS write timeout | 10s | 单次写入超时 |
| WS pong wait | 60s | 心跳超时 |

### 5.5 Key 校验

```go
var roomIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)
```

校验失败时生成 `crypto/rand` 的 16 字节 hex 随机串。

### 5.6 速率限制

```go
type RateLimiter struct {
    mu      sync.Mutex
    visitors map[string]*rate.Limiter // IP → limiter
    rate    rate.Limit
    burst   int
}
```

使用 `golang.org/x/time/rate`，per-IP 令牌桶。超限返回 HTTP 429 / WS close 1008 Policy Violation。

### 5.7 心跳

Go gorilla/websocket 内置 `SetPingHandler` / `SetPongHandler`:
- 服务端每 30s 发 ping
- 60s 内无 pong → terminate 连接
- 连接结构体维护 `lastPong time.Time`

### 5.8 房间清理

后台 goroutine 每 5 分钟扫描一次，`time.Now().After(room.expiry)` 的房间直接删除。有活动（任何消息）时刷新 `expiry = now + TTL`。

### 5.9 HTTP 端点

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/healthz` | 探活 → 200 OK |
| GET | `/s/` | 静态文件（index.html） |
| GET | `/s/ws` | WebSocket 升级 |
| GET | `/s/{roomId}` | 带房间号的页面 |

### 5.10 Middleware 链

```
Request →recover →log →rate limit →handler
```

WS 升级前执行 `CheckOrigin`:
```go
func CheckOrigin(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    return origin == "" || strings.HasPrefix(origin, "https://eu-as.cn")
}
```

### 5.11 优雅关闭

```go
signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
<-sig
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
server.Shutdown(ctx)
// 关闭所有 WS 并发 close frame
```

---

## 6. 前端设计

### 6.1 HTML 结构

```html
<body>
  <textarea id="editor" autofocus placeholder="开始输入..."></textarea>
  <div id="status">○</div>

  <div id="pairing-screen">
    <h2>📡 咕咕同步</h2>
    <div class="hint">手机扫码加入同一房间</div>
    <div id="qrcode"></div>
    <div id="pairStatus">等待连接...</div>
  </div>

  <div id="bottom-bar">
    <button class="bar-btn" id="cut-btn" title="剪切">
    <button class="bar-btn" id="copy-btn" title="复制">
    <button class="bar-btn" id="theme-btn" title="主题">
    <a class="bar-link" href="..." target="_blank">关于</a>
    <div class="spacer"></div>
    <span class="bar-meta">京ICP备...</span>
    <span class="bar-meta">京公网安备...</span>
  </div>
</body>
```

### 6.2 底部栏 Mobile 交互（核心修复）

**设计目标**: 键盘升起时，备案号不占用空间，操作按钮保持在键盘上方。

```css
/* 移动端: 备案号不渲染 */
@media (max-width: 767px) {
  .bar-meta { display: none; }
}

/* 键盘升起: 底部栏浮动覆盖且只显示操作按钮 */
.keyboard-open .bottom-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 100;
}
```

**键盘检测**: 保留 `visualViewport` API 方案，阈值 `diff > 80px` 判定键盘升起。

### 6.3 主题系统

- 默认: `dark`
- 切换: `dark ↔ light`
- 存储: `localStorage.theme`
- CSS 变量切换: `document.documentElement.dataset.theme`

### 6.4 状态机

```
状态: connected, paired, isPairingMode, isRePairing, isPeerUpdate

初始 → isPairingMode=true → 显示配对页 + QR
  ↓ 收到 {"t":"j"}
paired=true → 隐藏配对页 → 显示编辑器
  ↓ 收到 {"t":"l","c":1}
isRePairing=true → 显示重配覆盖层（保留编辑器文本）
  ↓ 收到 {"t":"j"}
paired=true → 回到编辑模式
```

### 6.5 输入同步

- `input` 事件 → 200ms debounce → WS 发送 `{d, p}`
- 收到远端消息 → `isPeerUpdate=true` → 更新 textarea + 光标 → `isPeerUpdate=false`
- 光标同步: `setSelectionRange(clampedPos, clampedPos)`

### 6.6 重连

WS 断开后 3s 自动重连，指数退避上限 30s。

### 6.7 唤醒锁

`navigator.wakeLock.request('screen')`，visibilitychange 时重新请求。

### 6.8 视觉风格

- 暗色默认背景 `#1a1a1a`，文字 `#e0e0e0`
- 编辑器无边框、无 padding 视觉框架，文字从屏幕边缘开始
- 底部栏: icon button 风格（无框、圆角 8px、hover 用背景色变化）
- 状态点: 8px 圆点，右上角，绿/红
- 配对页: 毛玻璃卡片 `backdrop-filter: blur(12px)`，QR 白底内嵌
- 关于链接: 文字链风格，不抢空间

---

## 7. 部署

### 7.1 构建

```bash
# 本地交叉编译
CGO_ENABLED=0 GOOS=linux go build -o sync-pad ./cmd/server

# 部署
scp sync-pad abj:/opt/sync-server/
ssh abj "sudo systemctl restart sync-pad"
```

### 7.2 systemd

```ini
# /etc/systemd/system/sync-pad.service
[Unit]
Description=Sync Pad WebSocket Server
After=network.target

[Service]
Type=simple
ExecStart=/opt/sync-server/sync-pad
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

### 7.3 Nginx 配置（修正后）

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

# 静态文件
location /s/ {
    alias /var/www/sync-text/;
    index index.html;
    try_files $uri /s/index.html;
}

# 主站 fallback 修正（关键修复）
location / {
    try_files $uri $uri/ =404;
}
```

### 7.4 Makefile

```makefile
.PHONY: build deploy run test clean

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o sync-pad ./cmd/server

deploy: build
	scp sync-pad abj:/opt/sync-server/
	ssh abj "sudo systemctl restart sync-pad"

run:
	go run ./cmd/server

test:
	go test ./...

clean:
	rm -f sync-pad
```

---

## 8. 开源准备

### 8.1 LICENSE

MIT License — 最宽松，适合引流项目。

### 8.2 CONTRIBUTING.md

- 如何本地运行
- 代码风格（gofmt / prettier）
- PR 流程

### 8.3 README.md 重写

- 项目简介 + 截图
- 快速开始（clone → make run）
- 部署指南
- 协议说明
- 贡献指南

### 8.4 GitHub Actions

```yaml
# .github/workflows/ci.yml
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go build ./...
      - run: go test ./...
      - run: go vet ./...
```

---

## 9. 实施顺序

| 阶段 | 内容 | 产出 |
|---|---|---|
| 1. Go 后端 | 项目骨架 + WS + 房间管理 + 安全 | 可运行的 Go 服务 |
| 2. 前端分离 | HTML/CSS/JS 拆分 + 状态机 | 可运行的前端 |
| 3. UI 重设计 | 视觉修复 + 底部栏交互 | 视觉可用的前端 |
| 4. 部署切换 | systemd + Nginx 修正 + 上线 | 生产环境切换 |
| 5. 开源准备 | LICENSE + CI + README + CONTRIBUTING | 可公开的开源项目 |

---

## 10. 风险与权衡

| 风险 | 缓解 |
|---|---|
| Go 学习曲线 | 代码量小（~500 行），标准库为主 |
| 协议兼容性 | 消息格式完全不变，旧前端可直连新后端 |
| 单节点故障 | 小规模可接受，systemd 自动重启 |
| 内存泄漏 | TTL 清理 + 连接上限兜底 |
| 前端分离后无构建 | 纯静态文件，Go 直接 serve，零构建步骤 |
