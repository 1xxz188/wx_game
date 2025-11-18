package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"testing"

	"wx_game/msg"
)

// 测试说明：
// 1. 测试需要在开发模式下运行（config.yaml 中 dev_mode: true）
// 2. 登录接口有速率限制（每分钟最多5次），如果测试失败提示"请求过于频繁"，
//    请等待1分钟后重试，或者减少并行运行的测试数量
// 3. 所有测试都使用 HTTP，开发模式下服务器使用 HTTP（默认地址：127.0.0.1:8080）
// 4. 测试需要服务器正在运行，可通过 `go run .` 启动

const (
	// 测试服务器地址
	testServerAddr = "43.100.128.210:8080"
)

// ========== WebSocket 测试示例 ==========

// getTestToken 辅助函数：获取测试用 token
// 注意：登录接口有速率限制（每分钟最多5次），如果测试失败提示"请求过于频繁"，
// 请等待1分钟后重试，或者减少并行运行的测试数量
func getTestToken(t *testing.T) string {
	// 使用开发模式的假 code 进行登录
	loginData := map[string]string{
		"code":      "fake-code-for-test",
		"device_id": "test-device-001",
	}
	jsonData, err := json.Marshal(loginData)
	assert.NoError(t, err)

	resp, err := http.Post(
		"http://"+testServerAddr+"/api/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if !assert.NoError(t, err, "登录请求失败，请确保服务器正在运行（开发模式 dev_mode: true）") {
		t.FailNow()
	}
	defer resp.Body.Close()

	// 读取响应体以便调试
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		t.Logf("登录请求被速率限制（429），响应: %s", string(body))
		t.Logf("提示：登录接口有速率限制（每分钟最多5次），请等待1分钟后重试")
		t.FailNow()
	}
	if resp.StatusCode != http.StatusOK {
		t.Logf("登录失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
		t.Logf("提示：请确保服务器在开发模式（dev_mode: true）下运行")
		t.FailNow()
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if !assert.NoError(t, err, "解析登录响应失败，响应体: %s", string(body)) {
		t.FailNow()
	}

	tokenVal, exists := result["token"]
	if !assert.True(t, exists, "响应中不存在 token 字段，响应: %v", result) {
		t.FailNow()
	}

	token, ok := tokenVal.(string)
	if !assert.True(t, ok, "token 类型错误，期望 string，实际: %T", tokenVal) {
		t.FailNow()
	}

	if !assert.NotEmpty(t, token, "token 为空") {
		t.FailNow()
	}

	return token
}

// TestWebSocketAuth 测试 WebSocket 连接和认证
func TestWebSocketAuth(t *testing.T) {
	// 0. 检查服务器是否运行（通过尝试登录）
	// 注意：使用 POST 方法检查，GET 方法会返回 405 Method Not Allowed
	loginData := map[string]string{
		"code":      "fake-code-for-test",
		"device_id": "test-check",
	}
	jsonData, _ := json.Marshal(loginData)
	resp, err := http.Post("http://"+testServerAddr+"/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Skipf("服务器未运行，跳过测试。请先启动服务器（开发模式 dev_mode: true）: %v", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	// 1. 获取 token
	token := getTestToken(t)
	fmt.Println("✓ 获取 token 成功")

	// 2. 连接 WebSocket
	wsURL := "ws://" + testServerAddr + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v\n提示：请确保服务器正在运行并且 WebSocket 路由已正确配置", err)
	}
	defer conn.Close()
	fmt.Println("✓ WebSocket 连接成功")

	// 3. 发送认证消息（Protobuf 格式）
	authMsg := &msg.Auth_Request{
		Token: token,
	}
	msgID := msg.LoginMsgAuth
	err = writeProtobufMessage(conn, MessageID(msgID), authMsg)
	assert.NoError(t, err, "发送认证消息失败")
	fmt.Println("✓ 发送认证消息成功")

	// 4. 接收认证响应
	respData, respMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err, "读取认证响应失败")
	assert.Equal(t, MessageID(msgID), respMsgID, "响应消息 ID 不匹配")

	var authResp msg.Auth_Response
	err = proto.Unmarshal(respData, &authResp)
	assert.NoError(t, err, "解析认证响应失败")
	assert.Equal(t, 0, authResp.Code, "认证失败，错误码: %d", authResp.Code)
	assert.Equal(t, "authenticated", authResp.Status)
	fmt.Printf("✓ 认证成功: %s\n", authResp.Status)
}

// TestWebSocketPing 测试 WebSocket Ping/Pong
func TestWebSocketPing(t *testing.T) {
	// 1. 获取 token 并连接
	token := getTestToken(t)
	conn := dialAndAuth(t, token)
	defer conn.Close()

	// 2. 发送 ping 消息
	pingMsg := &msg.Ping_Request{}
	msgID := msg.LoginMsgPing
	err := writeProtobufMessage(conn, MessageID(msgID), pingMsg)
	assert.NoError(t, err)
	fmt.Println("✓ 发送 ping 消息")

	// 3. 接收 pong 响应
	respData, respMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err)
	assert.Equal(t, MessageID(msgID), respMsgID, "响应消息 ID 不匹配")

	var pongResp msg.Ping_Response
	err = proto.Unmarshal(respData, &pongResp)
	assert.NoError(t, err)
	assert.Equal(t, 0, pongResp.Code, "ping 失败，错误码: %d", pongResp.Code)
	fmt.Println("✓ 收到 pong 响应")
}

// TestWebSocketAuthWithFirstMessage 测试在第一条消息中携带 token 并执行操作
func TestWebSocketAuthWithFirstMessage(t *testing.T) {
	// 0. 检查服务器是否运行
	loginData := map[string]string{
		"code":      "fake-code-for-test",
		"device_id": "test-check",
	}
	jsonData, _ := json.Marshal(loginData)
	resp, err := http.Post("http://"+testServerAddr+"/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Skipf("服务器未运行，跳过测试。请先启动服务器（开发模式 dev_mode: true）: %v", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	// 1. 获取 token
	token := getTestToken(t)

	// 2. 连接 WebSocket
	wsURL := "ws://" + testServerAddr + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v\n提示：请确保服务器正在运行并且 WebSocket 路由已正确配置", err)
	}
	defer conn.Close()
	fmt.Println("✓ WebSocket 连接成功")

	// 3. 先发送认证消息
	authMsg := &msg.Auth_Request{
		Token: token,
	}
	authMsgID := msg.LoginMsgAuth
	err = writeProtobufMessage(conn, MessageID(authMsgID), authMsg)
	assert.NoError(t, err)
	fmt.Println("✓ 发送认证消息")

	// 4. 接收认证响应
	authRespData, authRespMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err)
	assert.Equal(t, MessageID(authMsgID), authRespMsgID, "响应消息 ID 不匹配")
	var authResp msg.Auth_Response
	err = proto.Unmarshal(authRespData, &authResp)
	assert.NoError(t, err)
	assert.Equal(t, 0, authResp.Code)
	fmt.Println("✓ 认证成功")
}

// ========== 辅助函数 ==========

// dialAndAuth 连接 WebSocket 并完成认证
func dialAndAuth(t *testing.T, token string) *websocket.Conn {
	// 连接 WebSocket
	wsURL := "ws://" + testServerAddr + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v\n提示：请确保服务器正在运行并且 WebSocket 路由已正确配置", err)
	}

	// 发送认证消息（Protobuf 格式）
	authMsg := &msg.Auth_Request{
		Token: token,
	}
	msgID := msg.LoginMsgAuth
	err = writeProtobufMessage(conn, MessageID(msgID), authMsg)
	assert.NoError(t, err, "发送认证消息失败")

	// 接收认证响应
	respData, respMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err, "读取认证响应失败")
	assert.Equal(t, MessageID(msgID), respMsgID, "响应消息 ID 不匹配")

	var authResp msg.Auth_Response
	err = proto.Unmarshal(respData, &authResp)
	assert.NoError(t, err, "解析认证响应失败")
	assert.Equal(t, 0, authResp.Code, "认证失败，错误码: %d", authResp.Code)

	return conn
}

// writeProtobufMessage 写入 Protobuf 消息（消息头 + 数据）
func writeProtobufMessage(conn *websocket.Conn, msgID MessageID, msg proto.Message) error {
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
	return conn.WriteMessage(websocket.BinaryMessage, fullMessage)
}

// readProtobufMessage 读取 Protobuf 消息（返回数据部分和消息 ID）
func readProtobufMessage(conn *websocket.Conn) ([]byte, MessageID, error) {
	// 读取消息
	messageType, msgBytes, err := conn.ReadMessage()
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
	msgID := MessageID(binary.BigEndian.Uint32(msgBytes[:4]))

	// 提取 protobuf 数据（4 字节之后）
	protoData := msgBytes[4:]

	return protoData, msgID, nil
}

// truncateString 截断字符串用于显示
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
