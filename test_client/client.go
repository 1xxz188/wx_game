package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/msg/msg_id"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// Client 模拟客户端
type Client struct {
	serverAddr string
	token      string
	conn       *websocket.Conn
}

// NewClient 创建新的客户端实例
func NewClient(serverAddr string) *Client {
	return &Client{
		serverAddr: serverAddr,
	}
}

// GetToken 获取测试用 token
// 注意：登录接口有速率限制（每分钟最多5次）
func (c *Client) GetToken() error {
	// 使用开发模式的假 code 进行登录
	loginData := map[string]string{
		"code":      "fake-code-for-test",
		"device_id": "test-device-001",
	}
	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("序列化登录数据失败: %w", err)
	}

	resp, err := http.Post(
		"http://"+c.serverAddr+"/api/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("登录请求失败，请确保服务器正在运行（开发模式 dev_mode: true）: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体以便调试
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("登录请求被速率限制（429），响应: %s\n提示：登录接口有速率限制（每分钟最多5次），请等待1分钟后重试", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("登录失败，状态码: %d, 响应: %s\n提示：请确保服务器在开发模式（dev_mode: true）下运行", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return fmt.Errorf("解析登录响应失败，响应体: %s: %w", string(body), err)
	}

	tokenVal, exists := result["token"]
	if !exists {
		return fmt.Errorf("响应中不存在 token 字段，响应: %v", result)
	}

	token, ok := tokenVal.(string)
	if !ok {
		return fmt.Errorf("token 类型错误，期望 string，实际: %T", tokenVal)
	}

	if token == "" {
		return fmt.Errorf("token 为空")
	}

	c.token = token
	logger.Info("✓ Token obtained successfully")
	return nil
}

// Connect 连接 WebSocket 并完成认证
func (c *Client) Connect() error {
	if c.token == "" {
		return fmt.Errorf("token 为空，请先调用 GetToken()")
	}

	// 连接 WebSocket
	wsURL := "ws://" + c.serverAddr + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %w\n提示：请确保服务器正在运行并且 WebSocket 路由已正确配置", err)
	}
	c.conn = conn
	logger.Info("✓ WebSocket connection established")

	// 发送认证消息（Protobuf 格式）
	authMsg := &msg.LoginAuthRequest{
		Token: c.token,
	}
	msgID := msg_id.LoginAuth
	err = c.writeProtobufMessage(fw.MessageID(msgID), authMsg)
	if err != nil {
		conn.Close()
		return fmt.Errorf("发送认证消息失败: %w", err)
	}
	logger.Info("✓ Authentication message sent successfully")

	// 接收认证响应
	respData, respMsgID, err := c.readProtobufMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("读取认证响应失败: %w", err)
	}
	if respMsgID != fw.MessageID(msgID) {
		conn.Close()
		return fmt.Errorf("响应消息 ID 不匹配，期望: %d, 实际: %d", msgID, respMsgID)
	}

	var authResp msg.LoginAuthResponse
	err = proto.Unmarshal(respData, &authResp)
	if err != nil {
		conn.Close()
		return fmt.Errorf("解析认证响应失败: %w", err)
	}
	if authResp.Code != 0 {
		conn.Close()
		return fmt.Errorf("认证失败，错误码: %d", authResp.Code)
	}

	logger.Infof("✓ Authentication successful: %s", authResp.Status)
	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CallRPC 发送 RPC 请求并接收响应
func (c *Client) CallRPC(msgID int32, req proto.Message, resp proto.Message) error {
	if c.conn == nil {
		return fmt.Errorf("连接未建立，请先调用 Connect()")
	}

	err := c.writeProtobufMessage(fw.MessageID(msgID), req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}

	// 接收响应
	respData, respMsgID, err := c.readProtobufMessage()
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}
	if respMsgID != fw.MessageID(msgID) {
		return fmt.Errorf("响应消息 ID 不匹配，期望: %d, 实际: %d", msgID, respMsgID)
	}

	err = proto.Unmarshal(respData, resp)
	if err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	return nil
}

// SendRPC 只发送 RPC 请求（不等待响应）
func (c *Client) SendRPC(msgID int32, req proto.Message) error {
	if c.conn == nil {
		return fmt.Errorf("连接未建立，请先调用 Connect()")
	}

	err := c.writeProtobufMessage(fw.MessageID(msgID), req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}

	return nil
}

// ReceiveRPC 只接收 RPC 响应
func (c *Client) ReceiveRPC(msgID int32, resp proto.Message) error {
	if c.conn == nil {
		return fmt.Errorf("连接未建立，请先调用 Connect()")
	}

	// 接收响应
	respData, respMsgID, err := c.readProtobufMessage()
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}
	if respMsgID != fw.MessageID(msgID) {
		return fmt.Errorf("响应消息 ID 不匹配，期望: %d, 实际: %d", msgID, respMsgID)
	}

	err = proto.Unmarshal(respData, resp)
	if err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	return nil
}

// writeProtobufMessage 写入 Protobuf 消息（消息头 + 数据）
func (c *Client) writeProtobufMessage(msgID fw.MessageID, msg proto.Message) error {
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

	// 发送二进制消息
	return c.conn.WriteMessage(websocket.BinaryMessage, fullMessage)
}

// readProtobufMessage 读取 Protobuf 消息（返回数据部分和消息 ID）
func (c *Client) readProtobufMessage() ([]byte, fw.MessageID, error) {
	// 读取消息
	messageType, msgBytes, err := c.conn.ReadMessage()
	if err != nil {
		return nil, 0, err
	}

	if messageType != websocket.BinaryMessage {
		return nil, 0, fmt.Errorf("expected binary message, got %d", messageType)
	}

	if len(msgBytes) < 4 {
		return nil, 0, fmt.Errorf("message too short")
	}

	// 提取消息 ID（前 4 字节）
	msgID := fw.MessageID(binary.BigEndian.Uint32(msgBytes[:4]))

	// 提取 protobuf 数据（4 字节之后）
	protoData := msgBytes[4:]

	return protoData, msgID, nil
}

// GetConn 获取底层 WebSocket 连接（用于高级用法）
func (c *Client) GetConn() *websocket.Conn {
	return c.conn
}

// GetToken 获取当前 token
func (c *Client) GetTokenString() string {
	return c.token
}
