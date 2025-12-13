# Test Client

模拟客户端实现，用于测试游戏服务器功能。

## 功能特性

- 自动获取 Token（通过 HTTP 登录接口）
- WebSocket 连接和认证
- Protobuf 消息发送和接收
- RPC 调用（同步和异步模式）
- 完整的西瓜游戏流程示例

## 使用方法

### 基本用法

```go
package main

import (
    "fmt"
    "wx_game/test_client"
)

func main() {
    // 创建客户端
    client := test_client.NewClient("127.0.0.1:8080")
    
    // 获取 token
    err := client.GetToken()
    if err != nil {
        fmt.Printf("获取 token 失败: %v\n", err)
        return
    }
    
    // 连接并认证
    err = client.Connect()
    if err != nil {
        fmt.Printf("连接失败: %v\n", err)
        return
    }
    defer client.Close()
    
    // 发送 RPC 请求
    req := &msg.Ping_Request{}
    resp := &msg.Ping_Response{}
    err = client.CallRPC(msg.LoginMsgPing, req, resp)
    if err != nil {
        fmt.Printf("RPC 调用失败: %v\n", err)
        return
    }
    
    fmt.Printf("Ping 响应: %v\n", resp)
}
```

### 运行示例

```go
package main

import (
    "fmt"
    "wx_game/test_client"
)

func main() {
    err := test_client.ExampleWatermelon()
    if err != nil {
        fmt.Printf("示例运行失败: %v\n", err)
    }
}
```

## API 说明

### Client 结构体

- `NewClient(serverAddr string) *Client` - 创建新的客户端实例
- `GetToken() error` - 获取测试用 token
- `Connect() error` - 连接 WebSocket 并完成认证
- `Close() error` - 关闭连接
- `CallRPC(msgID int32, req proto.Message, resp proto.Message) error` - 发送 RPC 请求并接收响应
- `SendRPC(msgID int32, req proto.Message) error` - 只发送 RPC 请求（不等待响应）
- `ReceiveRPC(msgID int32, resp proto.Message) error` - 只接收 RPC 响应
- `GetConn() *websocket.Conn` - 获取底层 WebSocket 连接
- `GetTokenString() string` - 获取当前 token

## 注意事项

1. 登录接口有速率限制（每分钟最多5次），如果测试失败提示"请求过于频繁"，请等待1分钟后重试
2. 测试需要在开发模式下运行（config.yaml 中 dev_mode: true）
3. 所有测试都使用 HTTP，开发模式下服务器使用 HTTP（默认地址：127.0.0.1:8080）
4. 测试需要服务器正在运行，可通过 `go run .` 启动

