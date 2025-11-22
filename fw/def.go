package fw

import (
	"github.com/gofiber/websocket/v2"
	"google.golang.org/protobuf/proto"
	"sync"
)

// MessageID 消息 ID 类型，固定 4 字节（uint32）
type MessageID uint32
type ObjID int64

// MessageHandler 消息处理函数类型
// 参数：连接、消息 ID、反序列化后的 protobuf 消息、连接上下文
// 返回：响应消息（proto.Message）和错误
// 注意：ctx 参数是连接上下文的指针，处理函数可以修改它（例如设置 openID）
type MessageHandler func(c *websocket.Conn, msgID MessageID, msg proto.Message, ctx *ConnectionContext) (proto.Message, error)

// MessageInfo 消息信息，包含工厂函数和处理函数
type MessageInfo struct {
	Factory func() proto.Message // 创建空消息实例的工厂函数
	Handler MessageHandler       // 消息处理函数
}

type MsgInterface interface {
	Register(msgID MessageID, factory func() proto.Message, handler MessageHandler)
}

// SafeConn 线程安全的 WebSocket 连接包装器
// 确保同一时间只有一个 goroutine 执行写入操作
type SafeConn struct {
	Conn  *websocket.Conn
	mutex sync.Mutex
}

// WriteMessage 线程安全的消息写入方法
func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	return sc.Conn.WriteMessage(messageType, data)
}

// ConnectionContext 连接上下文，存储每个连接的状态信息
type ConnectionContext struct {
	ConnectionID  string // 连接唯一标识符
	OpenID        string
	Authenticated bool
	DeviceID      string
	SafeConn      *SafeConn // 线程安全的连接包装器
}
