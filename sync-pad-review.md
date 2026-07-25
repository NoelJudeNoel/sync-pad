# Sync Pad 深度评估报告

**仓库**: https://github.com/NoelJudeNoel/sync-pad
**评估日期**: 2026-07-20
**版本**: main 分支 (commit 73e7c860)
**线上**: https://eu-as.cn/s/

---

## 目录

1. [项目概览](#1-项目概览)
2. [架构评价](#2-架构评价)
3. [Bug 与安全缺陷](#3-bug-与安全缺陷)
4. [中等问题](#4-中等问题)
5. [做得好的地方](#5-做得好的地方)
6. [代码质量评分](#6-代码质量评分)
7. [建议优先修复](#7-建议优先修复)
8. [总结](#8-总结)

---

## 1. 项目概览

| 项目 | 详情 |
|---|---|
| 名称 | 咕咕同步 (sync-pad) |
| 定位 | 手机语音输入 → WebSocket 实时同步到 PC 的纯文本通道 |
| 语言 | Go 17KB / JS 10.8KB / CSS 6.2KB / HTML 2.2KB |
| 依赖 | gorilla/websocket + golang.org/x/time/rate（仅2个） |
| 规模 | ~500行 Go + ~500行 JS，单二进制部署 |
| 创建 | 2026-07-19，10次提交 |
| 线上 | https://eu-as.cn/s/ |

核心定位是**纯同步通道**，不做 ASR，不做通用协作编辑器。手机是输入端，PC 是接收端。

---

## 2. 架构评价

### 架构图

```
PC 浏览器 ──WSS──→ Nginx (:443) ──反代──→ Go sync-pad (:8080)
手机浏览器 ──WSS──→ Nginx (:443) ──反代──→ Go sync-pad (:8080)
                            ↓
                     内存态房间管理 (纯转发，不落盘)
```

### 做得好的

- 极简架构：Nginx 反代 → Go :8080 → 内存态房间，零外部依赖，clone 即跑
- 代码分层清晰：`config / ratelimit / room / server` 四个 internal 包，职责分明
- Go 代码风格地道：slog 结构化日志、RWMutex 并发控制、middleware 链（recover→log→rateLimit）
- 优雅关闭、信号处理、后台 TTL 清理 goroutine 都到位
- 前端零构建步骤，vanilla JS 模块化拆分（app/websocket/room/ui/theme），IIFE 封装
- 有设计文档（spec），有测试，有 CI，有 Dockerfile，开源工程化完整

### 设计文档与实现的偏差

| spec 写的 | 实际代码 | 影响 |
|---|---|---|
| pong wait 60s | 6s | 实际更激进，commit 里改过，是改进 |
| RateLimit msg 100/min | **未实现** | 🔴 见 §3 |
| MaxConnections 500 | **未实现** | 🔴 见 §3 |
| WS close 1008 超限 | 返回 HTTP 429 | 不一致 |

---

## 3. Bug 与安全缺陷

### 🔴 1. MaxConnections 完全未执行

**严重度**: 高

`config.go` 定义了 `MaxConnections: 500`，`config.Load()` 读取了它，但**全代码库没有任何地方检查这个值**。服务器会无限接受连接。

**相关代码**:

```go
// internal/config/config.go — 定义了但只是摆设
MaxConnections: 500,

// internal/server/server.go — HandleWS() 入口处没有任何连接数检查
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
    conn, err := s.upgrader.Upgrade(w, r, nil)  // ← 直接升级，无上限检查
    // ...
}
```

**修复建议**:

在 `HandleWS` 升级前检查全局连接数：

```go
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
    if s.rooms.TotalClientCount() >= s.cfg.MaxConnections {
        http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
        return
    }
    conn, err := s.upgrader.Upgrade(w, r, nil)
    // ...
}
```

需要在 `room.Manager` 上加一个 `TotalClientCount()` 方法，遍历所有 room 的 client 数求和（或维护一个 atomic counter）。

---

### 🔴 2. 消息速率限制器创建后从未使用

**严重度**: 高

```go
// internal/server/server.go NewApp() 第33行
msgLimit: ratelimit.New(rate.Limit(cfg.RateLimitMessages), cfg.RateLimitMessages/5),
```

`msgLimit` 被创建了，但 `readPump` 里收到消息时**从未调用 `a.msgLimit.Allow(ip)`**。配置说 100 msg/min per IP，实际上一个 IP 可以无限制地发送消息。

**相关代码**:

```go
// internal/server/websocket.go readPump() — 消息循环中无速率检查
for {
    _, data, err := conn.ReadMessage()
    if err != nil { break }

    // ← 这里应该检查 msgLimit.Allow(ip)，但没有
    var msg struct {
        D string `json:"d"`
        P *int   `json:"p"`
    }
    // ...
}
```

**修复建议**:

在 `readPump` 中加入消息速率检查。需要把 IP 或 msgLimit 传入 Server/Client：

```go
func (s *Server) readPump(c *room.Client, conn *websocket.Conn) {
    defer s.disconnect(c)
    // ... existing setup ...

    for {
        _, data, err := conn.ReadMessage()
        if err != nil { break }

        // 消息速率限制
        if !s.msgLimit.Allow(c.IP) {
            slog.Warn("rate limit hit (msg)", "room", c.Room.ID)
            conn.WriteMessage(websocket.CloseMessage,
                websocket.FormatCloseMessage(1008, "Policy Violation: rate limit"))
            return
        }

        // ... existing message handling ...
    }
}
```

注意：当前 `Client` 结构体不存储 IP，需要在 `HandleWS` 创建 client 时传入。

---

### 🔴 3. X-Forwarded-For 信任导致速率限制绕过

**严重度**: 高

```go
// internal/ratelimit/limiter.go ExtractIP()
func ExtractIP(r *http.Request) string {
    xff := r.Header.Get("X-Forwarded-For")
    if xff != "" {
        return xff  // ← 整个 XFF 值当作 IP
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}
```

**问题**:
- 攻击者可以每个请求设不同的 `X-Forwarded-For` 值来绕过 per-IP 速率限制
- 把整个 XFF 字符串（可能包含多个逗号分隔 IP）当作 key，同一用户通过不同代理会得到不同的 "IP"
- 客户端可以伪造 XFF 头，伪造任意 IP 作为身份

**正确做法**:

Nginx 应配置 `proxy_set_header X-Real-IP $remote_addr;`，然后优先取 `X-Real-IP`。如果必须用 XFF，取**最右侧不可信的 IP**（即最左边的客户端 IP）：

```go
func ExtractIP(r *http.Request) string {
    // 优先信任反代设置的 X-Real-IP
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return strings.TrimSpace(xri)
    }
    // XFF: 取最左边的 IP（最原始的客户端）
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        parts := strings.Split(xff, ",")
        return strings.TrimSpace(parts[0])
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}
```

同时 Nginx 配置需要确保：

```nginx
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

---

### 🔴 4. 房间 ID 模运算偏差 + 无鉴权

**严重度**: 中（长度足够长时影响小）

**房间 ID 生成**:

```go
// internal/server/websocket.go generateRoomID()
func generateRoomID() string {
    b := make([]byte, 16)
    rand.Read(b)
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789"  // 36 chars
    result := make([]byte, 16)
    for i := range b {
        result[i] = chars[b[i]%byte(len(chars))]  // ← 256 mod 36 = 4，前4个字符概率高
    }
    return string(result)
}
```

**问题**:
- **模运算偏差**: 256 mod 36 = 4，chars 的前4个字符（a,b,c,d）出现概率比其余高约 11%。实际熵略低于 `36^16`，但对 16 字符长度影响很小
- **无鉴权**: 任何人知道 `?room=xxx` 就能加入房间读取/修改文本。spec 明确排除了认证，但这意味着 QR 码被拍照/截图后内容可被窃取

**修复建议（消除模偏差）**:

用 `crypto/rand` + 拒绝采样，或用 base32url 编码：

```go
func generateRoomID() string {
    b := make([]byte, 12) // 12 bytes → 16 base32 chars
    if _, err := rand.Read(b); err != nil {
        panic(err)
    }
    return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}
```

对于无鉴权问题，应在 README 安全章节明确说明：房间 ID 是唯一凭证，不要分享给不信任的人。

---

### 🔴 5. Broadcast 静默丢消息

**严重度**: 中

```go
// internal/room/room.go Broadcast()
func (r *Room) Broadcast(sender *Client, msg []byte) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for c := range r.clients {
        if c != sender {
            select {
            case c.Send <- msg:
            default:
                // 通道满(16缓冲) → 消息被静默丢弃，无日志
            }
        }
    }
}
```

**问题**: 慢客户端会丢消息导致两端文本不同步。`BroadcastAll` 有同样问题。

**修复建议**:

丢消息时触发一次全量状态同步（发送当前 `rm.GetText()`），并记录日志：

```go
func (r *Room) Broadcast(sender *Client, msg []byte) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for c := range r.clients {
        if c != sender {
            select {
            case c.Send <- msg:
            default:
                slog.Warn("client send buffer full, dropping",
                    "room", r.ID, "msg_size", len(msg))
                // 触发全量重同步
                syncMsg, _ := json.Marshal(map[string]string{"d": r.Text})
                select {
                case c.Send <- syncMsg:
                default:
                    // 仍然满，只能放弃
                }
            }
        }
    }
}
```

---

## 4. 中等问题

### 🟡 6. Dockerfile 缺失配置提示

**严重度**: 中

```dockerfile
FROM alpine:3.19
# ...
ENV SYNC_PAD_WEB_DIR=/var/www/sync-pad
# 缺少: SYNC_PAD_ALLOWED_ORIGINS
CMD ["sync-pad"]
```

Docker 部署默认只允许 localhost origin，生产环境必须手动注入 `SYNC_PAD_ALLOWED_ORIGINS`，否则 WS 连接被拒绝。README 没提这个 Docker 陷阱。

**修复建议**:

```dockerfile
# 默认值用空字符串提示用户必须设置
ENV SYNC_PAD_ALLOWED_ORIGINS=""
ENV SYNC_PAD_WEB_DIR=/var/www/sync-pad

HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q -O- http://localhost:8080/healthz || exit 1
```

README Docker 部分加注：

> ⚠️ Docker 部署必须设置 `SYNC_PAD_ALLOWED_ORIGINS` 环境变量为你的域名，否则 WebSocket 连接会被拒绝。

---

### 🟡 7. setupVisibilityHandler 空转

**严重度**: 低

```js
// web/js/app.js init()
UI.setupVisibilityHandler();  // 没传 onHidden 回调
```

`ui.js` 里的 `setupVisibilityHandler(onHidden)` 接受一个回调，但调用时没传。实际效果只是释放 wakeLock，功能上没 bug 但代码意图不完整。

**修复建议**: 要么删除参数，要么传入有意义的回调（如断开 WS 节省资源）。

---

### 🟡 8. 无全量同步机制 / 无序列号

**严重度**: 中

当客户端丢消息（通道满）或重连后，只收到一次 `{"d":"<当前文本>"}`，但如果在断线期间另一端继续输入，重连后的文本是最新的，光标位置丢失。没有序列号/版本号来检测不一致。

**修复建议（长期）**:

在消息中加 `v`（version）字段：

```json
{"d":"文本", "p":5, "v":42}
```

客户端收到消息时检查 version 跳跃，如果不连续则请求一次全量同步。

---

### 🟡 9. 前端无输入长度限制

**严重度**: 低

后端 `MaxMessageSize: 64KB` 会在 `readPump` 里拒绝大消息，但前端 `sendIfDirty` 没有长度检查，用户粘贴大文本时会被服务端静默丢弃（只 `slog.Warn` 不回复错误）。

**修复建议**:

```js
// web/js/ui.js sendIfDirty()
function sendIfDirty(callback) {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
        const text = getEditorText();
        if (text.length > 60000) {  // 60KB 预警，留余量
            setPairStatus('⚠️ 文本过长，可能无法同步');
        }
        callback();
    }, 200);
}
```

---

### 🟡 10. 无 HEALTHCHECK 指令

**严重度**: 低

Dockerfile 没有 `HEALTHCHECK`，编排工具无法自动检测服务是否健康。`/healthz` 端点存在但 Docker 没用。

见 §4.6 的 Dockerfile 修复建议。

---

## 5. 做得好的地方

| 方面 | 评价 |
|---|---|
| **依赖管理** | 仅2个外部依赖，go.mod 干净 |
| **并发安全** | Room 和 Manager 都用 RWMutex 正确保护 |
| **测试覆盖** | room 和 ratelimit 有基础单测，CI 跑 build+test+vet |
| **优雅关闭** | 5s timeout + signal.Notify，正确 |
| **TTL 清理** | 后台 goroutine 定期清理过期房间，防止内存泄漏 |
| **Origin 校验** | WS upgrader 的 CheckOrigin 正确实现 |
| **前端重连** | 指数退避 3s→30s，合理 |
| **Wake Lock** | 移动端防止屏幕熄灭，visibilitychange 自动释放 |
| **键盘适配** | visualViewport 检测键盘升起，底部栏浮动 |
| **主题系统** | CSS 变量 + localStorage，dark/light 二态，简洁 |
| **QR 本地化** | qrcode.min.js 本地打包，不依赖 CDN |
| **零构建前端** | 无 npm/webpack，直接 `<script src>` 加载，符合"clone即跑" |

---

## 6. 代码质量评分

| 维度 | 分数 | 说明 |
|---|---|---|
| 架构设计 | 8/10 | 极简合理，分层清晰，有设计文档 |
| Go 代码质量 | 7/10 | 地道，但2个安全特性写了没用 |
| 前端代码质量 | 7/10 | 模块化好，但无错误处理、无长度校验 |
| 安全性 | 5/10 | XFF 信任 + MaxConn/MsgLimit 未执行 |
| 测试 | 6/10 | 基础单测有，但无集成测试、无 WS 端到端测试 |
| 部署/运维 | 7/10 | systemd+Nginx+Docker 三套方案，但 Docker 缺配置提示 |
| 文档 | 8/10 | README 详尽，有 spec、CONTRIBUTING、协议说明 |
| **综合** | **6.8/10** | 小而美的 MVP，但有"写了没用"的空壳安全特性 |

---

## 7. 建议优先修复

### P0 — 安全（必须修复）

```
1. 执行 MaxConnections 检查
   - 位置: internal/server/websocket.go HandleWS() 入口处
   - 方法: 在 WS 升级前检查全局连接数
   - 需要: room.Manager 加 TotalClientCount() 方法

2. 执行消息速率限制
   - 位置: internal/server/websocket.go readPump() 消息循环
   - 方法: 每条消息检查 msgLimit.Allow(ip)
   - 需要: Client 结构体存储 IP，或在 readPump 中传入

3. 修复 X-Forwarded-For 信任问题
   - 位置: internal/ratelimit/limiter.go ExtractIP()
   - 方法: 优先取 X-Real-IP，XFF 取最左边 IP
   - 需要: Nginx 配置 proxy_set_header X-Real-IP $remote_addr
```

### P1 — 可靠性（应该修复）

```
4. Broadcast 丢消息时触发全量重同步
   - 位置: internal/room/room.go Broadcast() 和 BroadcastAll()
   - 方法: default 分支中发送 r.Text 全量状态 + 记录日志

5. readPump 拒绝大消息时回复错误给客户端
   - 位置: internal/server/websocket.go readPump()
   - 方法: SetReadLimit 触发后发送 CloseMessage (1009 Message Too Big)

6. Dockerfile 加配置提示 + HEALTHCHECK
   - 位置: Dockerfile
   - 方法: ENV SYNC_PAD_ALLOWED_ORIGINS="" + HEALTHCHECK 指令
   - README 加 Docker 部署注意事项
```

### P2 — 体验（建议修复）

```
7. 前端加输入长度预检查 + 超限提示
   - 位置: web/js/ui.js sendIfDirty()
   - 方法: 检查 text.length > 60000 时显示警告

8. 重连后恢复光标位置
   - 位置: web/js/app.js handleMessage()
   - 方法: 收到 {"d":"..."} 时记录并恢复光标

9. 房间 ID 用 crypto/rand + base32url 消除模偏差
   - 位置: internal/server/websocket.go generateRoomID()
   - 方法: encoding/base32 + NoPadding

10. setupVisibilityHandler 传入有意义回调或删除参数
    - 位置: web/js/app.js init() + web/js/ui.js
```

---

## 8. 总结

这是一个**定位精准、架构干净的小工具**。"手机语音输入 → PC 实时同步"这个场景切得很准，不做 ASR/协作/持久化是正确的克制。Go 后端 + vanilla JS 前端 + 单二进制部署的选择让开源部署门槛极低，适合引流。

主要问题是**安全特性"形同虚设"**——MaxConnections 和 msgLimit 写了配置、建了对象，但从未在关键路径上调用。这不是设计缺陷而是实现遗漏，修复成本低。XFF 信任问题是部署在 Nginx 后面时最容易踩的安全坑。

总体来说，作为个人开源项目质量不错，修复 P0 三项后就是可以放心推荐别人部署的工具。

---

*报告生成 by Hermes Agent*
