package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
	cfgCode "wx_game/cfg/code"
	"wx_game/component"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/role"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"google.golang.org/protobuf/proto"
)

// ConnectionManager 连接管理器，用于维护在线连接和广播消息
type ConnectionManager struct {
	connections map[string]*fw.ConnectionContext // connectionID -> fw.ConnectionContext
	mutex       sync.RWMutex                     // 保护 connections map
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*fw.ConnectionContext),
	}
}

// Register 注册新连接
func (cm *ConnectionManager) Register(ctx *fw.ConnectionContext) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[ctx.ConnectionID] = ctx
	logger.Infof("new conn id[%s] online[%d]", ctx.ConnectionID, len(cm.connections))
}

// Unregister 注销连接
func (cm *ConnectionManager) Unregister(connectionID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.connections, connectionID)
	logger.Infof("rm conn id[%s] online[%d]", connectionID, len(cm.connections))
}

// GetConnection 获取指定连接
func (cm *ConnectionManager) GetConnection(connectionID string) (*fw.ConnectionContext, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	ctx, ok := cm.connections[connectionID]
	return ctx, ok
}

// GetAllConnections 获取所有连接（返回副本以避免并发问题）
func (cm *ConnectionManager) GetAllConnections() []*fw.ConnectionContext {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	result := make([]*fw.ConnectionContext, 0, len(cm.connections))
	for _, ctx := range cm.connections {
		result = append(result, ctx)
	}
	return result
}

// GetAuthenticatedConnections 获取所有已认证的连接
func (cm *ConnectionManager) GetAuthenticatedConnections() []*fw.ConnectionContext {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	result := make([]*fw.ConnectionContext, 0)
	for _, ctx := range cm.connections {
		if ctx.Authenticated {
			result = append(result, ctx)
		}
	}
	return result
}

// GetConnectionsByOpenID 根据 openID 获取连接列表
func (cm *ConnectionManager) GetConnectionsByOpenID(openID string) []*fw.ConnectionContext {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	result := make([]*fw.ConnectionContext, 0)
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
func (cm *ConnectionManager) Broadcast(msgID fw.MessageID, msg proto.Message) int {
	connections := cm.GetAllConnections()
	successCount := 0
	for _, ctx := range connections {
		if err := writeMessage(ctx, msgID, msg); err != nil {
			logger.Errorf("Failed to broadcast message (connectionID=%s): %v", ctx.ConnectionID, err)
			// 连接可能已断开，尝试注销
			cm.Unregister(ctx.ConnectionID)
		} else {
			successCount++
		}
	}
	return successCount
}

// BroadcastToAuthenticated 广播消息给所有已认证的连接
func (cm *ConnectionManager) BroadcastToAuthenticated(msgID fw.MessageID, msg proto.Message) int {
	connections := cm.GetAuthenticatedConnections()
	successCount := 0
	for _, ctx := range connections {
		if err := writeMessage(ctx, msgID, msg); err != nil {
			logger.Errorf("Failed to broadcast message to authenticated user (connectionID=%s, openID=%s): %v", ctx.ConnectionID, ctx.OpenID, err)
			// 连接可能已断开，尝试注销
			cm.Unregister(ctx.ConnectionID)
		} else {
			successCount++
		}
	}
	return successCount
}

// SendToOpenID 发送消息给指定 openID 的所有连接
func (cm *ConnectionManager) SendToOpenID(openID string, msgID fw.MessageID, msg proto.Message) int {
	connections := cm.GetConnectionsByOpenID(openID)
	successCount := 0
	for _, ctx := range connections {
		if err := writeMessage(ctx, msgID, msg); err != nil {
			logger.Infof("Failed to send message to user (connectionID=%s, openID=%s): %v", ctx.ConnectionID, ctx.OpenID, err)
			// 连接可能已断开，尝试注销
			cm.Unregister(ctx.ConnectionID)
		} else {
			successCount++
		}
	}
	return successCount
}

// writeMessage 写入消息：先写入 msgID（4 字节），再写入 protobuf 数据
// 使用 SafeConn 确保并发安全
func writeMessage(ctx *fw.ConnectionContext, msgID fw.MessageID, msg proto.Message) error {
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
	return ctx.SafeConn.WriteMessage(websocket.BinaryMessage, fullMessage)
}

// WSService WebSocket 服务结构体
type WSService struct {
	authService       *AuthService
	registry          *fw.MessageRegistry
	connectionManager *ConnectionManager // 连接管理器
	roleMgr           *role.Mgr

	muComp         sync.Mutex
	registeredComp map[string]component.Component
	handlerComp    []component.Component
}

// GetConnectionManager 获取连接管理器（用于外部调用广播功能）
func (ws *WSService) GetConnectionManager() *ConnectionManager {
	return ws.connectionManager
}

// NewWSService 创建 WebSocket 服务实例
func NewWSService(authService *AuthService, roleMgr *role.Mgr) *WSService {
	ws := &WSService{
		authService:       authService,
		registry:          fw.NewMessageRegistry(),
		connectionManager: NewConnectionManager(),
		registeredComp:    make(map[string]component.Component, 0),
		handlerComp:       make([]component.Component, 0),
		roleMgr:           roleMgr,
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
	ws.registry.Register(fw.MessageID(msg.LoginMsgAuth),
		func() proto.Message { return &msg.Auth_Request{} },
		ws.handleAuthRequest,
	)

	// 注册心跳请求
	ws.registry.Register(fw.MessageID(msg.LoginMsgPing),
		func() proto.Message { return &msg.Ping_Request{} },
		ws.handlePingRequest,
	)
}

func (ws *WSService) RegisterMsg(msgID fw.MessageID, factory func() proto.Message, handler fw.MessageHandler) {
	ws.registry.Register(msgID,
		factory,
		handler,
	)
}

// Handler 创建 WebSocket 处理器（支持消息 ID 路由）
func (ws *WSService) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// 生成唯一连接ID
		connectionID := generateConnectionID()

		// 创建线程安全的连接包装器
		SafeConn := &fw.SafeConn{Conn: c}

		// 创建连接上下文
		ctx := &fw.ConnectionContext{
			ConnectionID:  connectionID,
			Authenticated: false,
			SafeConn:      SafeConn,
		}

		// 注册连接
		ws.connectionManager.Register(ctx)

		// 确保在连接断开时注销
		defer func() {
			ws.connectionManager.Unregister(connectionID)
			if rec := recover(); rec != nil {
				stackTrace := debug.Stack()
				stackTraceAsRawStringLiteral := strconv.Quote(string(stackTrace))
				logger.Errorf("rec: %v, stackTrace: %v", rec, stackTraceAsRawStringLiteral)
			}
		}()

		// 消息循环
		for {
			// 读取消息类型
			messageType, msgBytes, err := c.ReadMessage()
			if err != nil {
				//logger.Warnf("WebSocket read error: %v", err)
				break
			}

			// 只处理二进制消息
			if messageType != websocket.BinaryMessage {
				// 无法发送错误响应（没有 msgID），记录日志即可
				logger.Errorf("id[%s] Received non-binary message, ignoring", connectionID)
				continue
			}

			// 检查消息长度（至少 4 字节消息头）
			if len(msgBytes) < 4 {
				logger.Errorf("id[%s] Message too short, ignoring", connectionID)
				continue
			}

			// 提取消息 ID（前 4 字节）
			msgID := fw.MessageID(binary.BigEndian.Uint32(msgBytes[:4]))

			// 提取 protobuf 数据（4 字节之后）
			protoData := msgBytes[4:]

			logger.Debugf("id[%s] msgType[%d] len[%d] msgID[%d]", connectionID, messageType, len(msgBytes), msgID)

			// 根据 msgID 获取消息信息（使用合并后的 map）
			msgInfo, ok := ws.registry.Get(msgID)
			if !ok {
				logger.Errorf("Unknown message ID: 0x%08X", msgID)
				continue
			}

			// 创建消息实例
			req := msgInfo.Factory()

			// 反序列化 protobuf 消息
			if err := proto.Unmarshal(protoData, req); err != nil {
				logger.Errorf("Protobuf deserialization failed (msgID=0x%08X): %v", msgID, err)
				continue
			}

			// 处理认证（除了认证请求本身）
			if msgID != fw.MessageID(msg.LoginMsgAuth) {
				if !ctx.Authenticated {
					// 需要认证，根据消息类型返回对应的错误响应
					resp := ws.createErrorResponse(msgID, int32(cfgCode.EErrorCode_AuthRequired), "authentication required")
					if resp != nil {
						if err := writeMessage(ctx, msgID, resp); err != nil {
							logger.Errorf("Failed to send authentication error response: %v", err)
						}
					}
					continue
				}
			}

			// 调用处理函数（传递连接上下文）
			resp, err := msgInfo.Handler(c, msgID, req, ctx)
			if err != nil {
				logger.Errorf("Failed to process message (msgID=0x%08X): %v", msgID, err)
				continue
			}

			// 如果处理函数返回了响应，发送响应（使用相同的 msgID）
			if resp != nil {
				if err := writeMessage(ctx, msgID, resp); err != nil {
					logger.Errorf("Failed to send response: %v", err)
				}
			}
		}

		logger.Debugf("WebSocket connection[%s] openID[%s] closed", ctx.ConnectionID, ctx.OpenID)
	})
}

// createErrorResponse 根据消息 ID 创建对应的错误响应消息
func (ws *WSService) createErrorResponse(msgID fw.MessageID, errorCode int32, errMsg string) proto.Message {
	switch msgID {
	case msg.LoginMsgAuth:
		return &msg.Auth_Response{
			Code:   errorCode,
			Status: errMsg,
		}
	case msg.LoginMsgPing:
		return &msg.Ping_Response{
			Code: errorCode,
		}
	default:
		return nil
	}
}

// ==================== 消息处理函数 ====================

// handleAuthRequest 处理认证请求
func (ws *WSService) handleAuthRequest(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.Auth_Request)

	if req.Token == "" {
		return &msg.Auth_Response{
			Code:   int32(cfgCode.EErrorCode_InvalidToken),
			Status: "token required",
		}, nil
	}

	parsedOpenID, deviceID, err := ws.authService.ParseToken(req.Token)
	if err != nil {
		return &msg.Auth_Response{
			Code:   int32(cfgCode.EErrorCode_AuthFailed),
			Status: "invalid token: " + err.Error(),
		}, nil
	}

	// 更新连接上下文
	ctx.OpenID = parsedOpenID
	ctx.DeviceID = deviceID
	ctx.Authenticated = true

	resp := &msg.Auth_Response{
		Code:   int32(cfgCode.EErrorCode_None),
		Status: "authenticated",
	}

	ws.roleMgr.LoginRole(ctx.OpenID, func(r *role.Info) {
		resp.Role = proto.Clone(r.Role.Base).(*msg.RoleBase)
	})

	ctx.RoleId = resp.Role.RoleId
	logger.Infof("role_id[%d] id[%s] open_id[%s] deviceID[%s] auth successful", ctx.RoleId, ctx.ConnectionID, parsedOpenID, deviceID)
	return resp, nil
}

// handlePingRequest 处理心跳请求
func (ws *WSService) handlePingRequest(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	return &msg.Ping_Response{
		Code: int32(cfgCode.EErrorCode_None),
	}, nil
}
