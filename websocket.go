package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
	"wx_game/msg"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"google.golang.org/protobuf/proto"
)

// MessageID 消息 ID 类型，固定 4 字节（uint32）
type MessageID uint32

// SafeConn 线程安全的 WebSocket 连接包装器
// 确保同一时间只有一个 goroutine 执行写入操作
type SafeConn struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

// WriteMessage 线程安全的消息写入方法
func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	return sc.conn.WriteMessage(messageType, data)
}

// ConnectionContext 连接上下文，存储每个连接的状态信息
type ConnectionContext struct {
	ConnectionID  string // 连接唯一标识符
	OpenID        string
	Authenticated bool
	DeviceID      string
	safeConn      *SafeConn // 线程安全的连接包装器
}

// ConnectionManager 连接管理器，用于维护在线连接和广播消息
type ConnectionManager struct {
	connections map[string]*ConnectionContext // connectionID -> ConnectionContext
	mutex       sync.RWMutex                  // 保护 connections map
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*ConnectionContext),
	}
}

// Register 注册新连接
func (cm *ConnectionManager) Register(ctx *ConnectionContext) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[ctx.ConnectionID] = ctx
	log.Printf("连接注册: connectionID=%s, 当前在线连接数: %d", ctx.ConnectionID, len(cm.connections))
}

// Unregister 注销连接
func (cm *ConnectionManager) Unregister(connectionID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.connections, connectionID)
	log.Printf("连接注销: connectionID=%s, 当前在线连接数: %d", connectionID, len(cm.connections))
}

// GetConnection 获取指定连接
func (cm *ConnectionManager) GetConnection(connectionID string) (*ConnectionContext, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	ctx, ok := cm.connections[connectionID]
	return ctx, ok
}

// GetAllConnections 获取所有连接（返回副本以避免并发问题）
func (cm *ConnectionManager) GetAllConnections() []*ConnectionContext {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	result := make([]*ConnectionContext, 0, len(cm.connections))
	for _, ctx := range cm.connections {
		result = append(result, ctx)
	}
	return result
}

// GetAuthenticatedConnections 获取所有已认证的连接
func (cm *ConnectionManager) GetAuthenticatedConnections() []*ConnectionContext {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	result := make([]*ConnectionContext, 0)
	for _, ctx := range cm.connections {
		if ctx.Authenticated {
			result = append(result, ctx)
		}
	}
	return result
}

// GetConnectionsByOpenID 根据 openID 获取连接列表
func (cm *ConnectionManager) GetConnectionsByOpenID(openID string) []*ConnectionContext {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	result := make([]*ConnectionContext, 0)
	for _, ctx := range cm.connections {
		if ctx.Authenticated && ctx.OpenID == openID {
			result = append(result, ctx)
		}
	}
	return result
}

// GetConnectionCount 获取当前在线连接数
func (cm *ConnectionManager) GetConnectionCount() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return len(cm.connections)
}

// Broadcast 广播消息给所有连接
func (cm *ConnectionManager) Broadcast(msgID MessageID, msg proto.Message) int {
	connections := cm.GetAllConnections()
	successCount := 0
	for _, ctx := range connections {
		if err := writeMessage(ctx, msgID, msg); err != nil {
			log.Printf("广播消息失败 (connectionID=%s): %v", ctx.ConnectionID, err)
			// 连接可能已断开，尝试注销
			cm.Unregister(ctx.ConnectionID)
		} else {
			successCount++
		}
	}
	return successCount
}

// BroadcastToAuthenticated 广播消息给所有已认证的连接
func (cm *ConnectionManager) BroadcastToAuthenticated(msgID MessageID, msg proto.Message) int {
	connections := cm.GetAuthenticatedConnections()
	successCount := 0
	for _, ctx := range connections {
		if err := writeMessage(ctx, msgID, msg); err != nil {
			log.Printf("广播消息给已认证用户失败 (connectionID=%s, openID=%s): %v", ctx.ConnectionID, ctx.OpenID, err)
			// 连接可能已断开，尝试注销
			cm.Unregister(ctx.ConnectionID)
		} else {
			successCount++
		}
	}
	return successCount
}

// SendToOpenID 发送消息给指定 openID 的所有连接
func (cm *ConnectionManager) SendToOpenID(openID string, msgID MessageID, msg proto.Message) int {
	connections := cm.GetConnectionsByOpenID(openID)
	successCount := 0
	for _, ctx := range connections {
		if err := writeMessage(ctx, msgID, msg); err != nil {
			log.Printf("发送消息给用户失败 (connectionID=%s, openID=%s): %v", ctx.ConnectionID, ctx.OpenID, err)
			// 连接可能已断开，尝试注销
			cm.Unregister(ctx.ConnectionID)
		} else {
			successCount++
		}
	}
	return successCount
}

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

// MessageRegistry 消息注册表（合并后的单一 map）
type MessageRegistry struct {
	// msgID -> 消息信息（包含工厂函数和处理函数）
	messages map[MessageID]*MessageInfo
}

// NewMessageRegistry 创建消息注册表
func NewMessageRegistry() *MessageRegistry {
	return &MessageRegistry{
		messages: make(map[MessageID]*MessageInfo),
	}
}

// Register 注册消息类型和处理函数
// msgID: 消息 ID（4 字节）
// factory: 创建空消息实例的函数，例如：func() proto.Message { return &msg.AuthRequest{} }
// handler: 消息处理函数
func (r *MessageRegistry) Register(msgID MessageID, factory func() proto.Message, handler MessageHandler) {
	r.messages[msgID] = &MessageInfo{
		Factory: factory,
		Handler: handler,
	}
}

// GetFactory 获取消息类型工厂函数
func (r *MessageRegistry) GetFactory(msgID MessageID) (func() proto.Message, bool) {
	info, ok := r.messages[msgID]
	if !ok {
		return nil, false
	}
	return info.Factory, true
}

// GetHandler 获取消息处理函数
func (r *MessageRegistry) GetHandler(msgID MessageID) (MessageHandler, bool) {
	info, ok := r.messages[msgID]
	if !ok {
		return nil, false
	}
	return info.Handler, true
}

// Get 获取完整的消息信息
func (r *MessageRegistry) Get(msgID MessageID) (*MessageInfo, bool) {
	info, ok := r.messages[msgID]
	return info, ok
}

// writeMessage 写入消息：先写入 msgID（4 字节），再写入 protobuf 数据
// 使用 SafeConn 确保并发安全
func writeMessage(ctx *ConnectionContext, msgID MessageID, msg proto.Message) error {
	// 序列化 protobuf 消息
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	// 准备消息头（4 字节 msgID） + 消息体（protobuf 数据）
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(msgID))

	// 合并消息头和消息体
	fullMessage := append(header, data...)

	// 使用线程安全的连接发送二进制消息
	return ctx.safeConn.WriteMessage(websocket.BinaryMessage, fullMessage)
}

// WSService WebSocket 服务结构体
type WSService struct {
	authService       *AuthService
	registry          *MessageRegistry
	connectionManager *ConnectionManager // 连接管理器
}

// GetConnectionManager 获取连接管理器（用于外部调用广播功能）
func (ws *WSService) GetConnectionManager() *ConnectionManager {
	return ws.connectionManager
}

// NewWSService 创建 WebSocket 服务实例
func NewWSService(authService *AuthService) *WSService {
	ws := &WSService{
		authService:       authService,
		registry:          NewMessageRegistry(),
		connectionManager: NewConnectionManager(),
	}

	// 注册所有消息类型和处理函数
	ws.registerMessages()

	return ws
}

// generateConnectionID 生成唯一的连接ID
func generateConnectionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 如果随机数生成失败，使用时间戳
		return fmt.Sprintf("conn_%d", time.Now().UnixNano())
	}
	return "conn_" + hex.EncodeToString(b)
}

// registerMessages 注册所有消息类型和处理函数
func (ws *WSService) registerMessages() {
	// 注册认证请求（使用 msg 生成的枚举）
	ws.registry.Register(MessageID(msg.MessageID_MSG_ID_AUTH_REQUEST),
		func() proto.Message { return &msg.Auth_Request{} },
		ws.handleAuthRequest,
	)

	// 注册心跳请求
	ws.registry.Register(MessageID(msg.MessageID_MSG_ID_PING_REQUEST),
		func() proto.Message { return &msg.Ping_Request{} },
		ws.handlePingRequest,
	)
}

// Handler 创建 WebSocket 处理器（支持消息 ID 路由）
func (ws *WSService) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// 生成唯一连接ID
		connectionID := generateConnectionID()

		// 创建线程安全的连接包装器
		safeConn := &SafeConn{conn: c}

		// 创建连接上下文
		ctx := &ConnectionContext{
			ConnectionID:  connectionID,
			Authenticated: false,
			safeConn:      safeConn,
		}

		// 注册连接
		ws.connectionManager.Register(ctx)

		// 确保在连接断开时注销
		defer func() {
			ws.connectionManager.Unregister(connectionID)
		}()

		log.Printf("WebSocket 连接建立（Protobuf + MessageID 协议）: connectionID=%s", connectionID)

		// 消息循环
		for {
			// 读取消息类型
			messageType, msgBytes, err := c.ReadMessage()
			if err != nil {
				log.Printf("WebSocket 读取错误: %v", err)
				break
			}

			// 只处理二进制消息
			if messageType != websocket.BinaryMessage {
				// 无法发送错误响应（没有 msgID），记录日志即可
				log.Printf("收到非二进制消息，忽略")
				continue
			}

			// 检查消息长度（至少 4 字节消息头）
			if len(msgBytes) < 4 {
				log.Printf("消息太短，忽略")
				continue
			}

			// 提取消息 ID（前 4 字节）
			msgID := MessageID(binary.BigEndian.Uint32(msgBytes[:4]))

			// 提取 protobuf 数据（4 字节之后）
			protoData := msgBytes[4:]

			// 根据 msgID 获取消息信息（使用合并后的 map）
			msgInfo, ok := ws.registry.Get(msgID)
			if !ok {
				log.Printf("未知的消息 ID: 0x%08X", msgID)
				continue
			}

			// 创建消息实例
			req := msgInfo.Factory()

			// 反序列化 protobuf 消息
			if err := proto.Unmarshal(protoData, req); err != nil {
				log.Printf("Protobuf 反序列化失败 (msgID=0x%08X): %v", msgID, err)
				continue
			}

			// 处理认证（除了认证请求本身）
			if msgID != MessageID(msg.MessageID_MSG_ID_AUTH_REQUEST) {
				if !ctx.Authenticated {
					// 需要认证，根据消息类型返回对应的错误响应
					resp := ws.createErrorResponse(msgID, int32(msg.ErrorCode_ERROR_CODE_AUTH_REQUIRED), "authentication required")
					if resp != nil {
						if err := writeMessage(ctx, msgID, resp); err != nil {
							log.Printf("发送认证错误响应失败: %v", err)
						}
					}
					continue
				}
			}

			// 调用处理函数（传递连接上下文）
			resp, err := msgInfo.Handler(c, msgID, req, ctx)
			if err != nil {
				log.Printf("处理消息失败 (msgID=0x%08X): %v", msgID, err)
				continue
			}

			// 如果处理函数返回了响应，发送响应（使用相同的 msgID）
			if resp != nil {
				if err := writeMessage(ctx, msgID, resp); err != nil {
					log.Printf("发送响应失败: %v", err)
				}
			}
		}

		log.Printf("WebSocket 断开连接: connectionID=%s, openID=%s", ctx.ConnectionID, ctx.OpenID)
	})
}

// createErrorResponse 根据消息 ID 创建对应的错误响应消息
func (ws *WSService) createErrorResponse(msgID MessageID, errorCode int32, errMsg string) proto.Message {
	switch msg.MessageID(msgID) {
	case msg.MessageID_MSG_ID_AUTH_REQUEST:
		return &msg.Auth_Response{
			Code:   errorCode,
			Status: errMsg,
		}
	case msg.MessageID_MSG_ID_PING_REQUEST:
		return &msg.Ping_Response{
			Code: errorCode,
		}
	default:
		return nil
	}
}

// ==================== 消息处理函数 ====================

// handleAuthRequest 处理认证请求
func (ws *WSService) handleAuthRequest(c *websocket.Conn, msgID MessageID, m proto.Message, ctx *ConnectionContext) (proto.Message, error) {
	req := m.(*msg.Auth_Request)

	if req.Token == "" {
		return &msg.Auth_Response{
			Code:   int32(msg.ErrorCode_ERROR_CODE_INVALID_TOKEN),
			Status: "token required",
		}, nil
	}

	parsedOpenID, deviceID, err := ws.authService.ParseToken(req.Token)
	if err != nil {
		return &msg.Auth_Response{
			Code:   int32(msg.ErrorCode_ERROR_CODE_AUTH_FAILED),
			Status: "invalid token: " + err.Error(),
		}, nil
	}

	// 更新连接上下文
	ctx.OpenID = parsedOpenID
	ctx.DeviceID = deviceID
	ctx.Authenticated = true

	log.Printf("WebSocket 认证成功: openID=%s deviceID=%s", parsedOpenID, deviceID)

	return &msg.Auth_Response{
		Code:   int32(msg.ErrorCode_ERROR_CODE_SUCCESS),
		Status: "authenticated",
	}, nil
}

// handlePingRequest 处理心跳请求
func (ws *WSService) handlePingRequest(c *websocket.Conn, msgID MessageID, m proto.Message, ctx *ConnectionContext) (proto.Message, error) {
	return &msg.Ping_Response{
		Code: int32(msg.ErrorCode_ERROR_CODE_SUCCESS),
	}, nil
}
