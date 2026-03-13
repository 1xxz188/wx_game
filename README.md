# wx_game - 微信小游戏后端服务

微信小游戏（西瓜游戏）后端服务，单体架构，纯 Go 实现。

## 技术栈

| 分类 | 技术 |
|------|------|
| 语言 | Go 1.25 |
| Web 框架 | Fiber v2.52（HTTP + WebSocket） |
| 数据库 | MongoDB（持久化）、Redis（预留） |
| 通信协议 | Protobuf 3（消息序列化） |
| 认证 | JWT + 微信小程序登录 |

## 目录结构

```
wx_game/
├── main.go          # 服务入口
├── handlers.go      # HTTP 路由（/api/login）
├── websocket.go     # WebSocket 连接管理与消息分发
├── auth.go          # JWT 认证
├── wechat.go        # 微信登录集成
├── config.yaml      # 服务配置
├── watermelon/      # 西瓜游戏核心逻辑
├── role/            # 玩家角色管理
├── rank/            # 排行榜
├── fw/              # 框架层（FSM、持久化、消息注册）
│   ├── fsm/         # 有限状态机
│   ├── mdzset/      # 多维有序集合
│   └── persistence/ # 数据持久化（MongoDB）
├── msg/protos/      # Protobuf 协议定义
└── cfg/             # 游戏配置表
```

## 核心业务流

```
微信登录 → JWT Token → WebSocket 连接
→ 消息认证 → 角色数据加载
→ 西瓜游戏交互（开始/掉落/合并/计分）
→ 排行榜更新 → MongoDB 持久化
```

## 架构特征

- **单体应用**：单进程内包含 HTTP + WebSocket 服务器
- **事件驱动**：WebSocket 消息通过消息注册表动态分发
- **状态机（FSM）**：管理游戏状态转换
- **并发安全**：Goroutine + concurrent-map 处理并发连接
- **分层设计**：表现层 → 业务逻辑层 → 持久化层 → 框架层

## 快速开始

### 配置

编辑 `config.yaml`，配置以下参数：

- 服务端口
- MongoDB 连接地址
- 微信小程序 AppID / AppSecret
- JWT 密钥
- 日志级别

### 编译运行

```bash
# Windows 调试构建
make_debug.bat

# Linux 构建
bash make_linux.bat

# 直接运行
go run .
```

## API

| 接口 | 协议 | 说明 |
|------|------|------|
| `/api/login` | HTTP POST | 微信登录，返回 JWT Token |
| `/ws` | WebSocket | 游戏实时通信 |
