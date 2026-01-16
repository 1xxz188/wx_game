package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/msg/msg_id"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// 测试说明：
// 1. 测试需要在开发模式下运行（config.yaml 中 dev_mode: true）
// 2. 登录接口有速率限制（每分钟最多5次），如果测试失败提示"请求过于频繁"，
//    请等待1分钟后重试，或者减少并行运行的测试数量
// 3. 所有测试都使用 HTTP，开发模式下服务器使用 HTTP（默认地址：127.0.0.1:8080）
// 4. 测试需要服务器正在运行，可通过 `go run .` 启动

const (
	// 测试服务器地址
	//testServerAddr = "43.100.128.210:8080"
	testServerAddr = "xxzos.xyz:443"
	//testServerAddr = "127.0.0.1:8080"
)

// ========== WebSocket 测试示例 ==========
// TestWebSocketAuth 测试 WebSocket 连接和认证
func TestWebSocketAuth(t *testing.T) {
	token := getTestToken(t)
	conn := dialAndAuth(t, token)
	defer conn.Close()
	logger.Info("✓ WebSocket 连接和认证测试通过")
}

// TestWebSocketPing 测试 WebSocket Ping/Pong
func TestWebSocketPing(t *testing.T) {
	// 1. 获取 token 并连接
	token := getTestToken(t)
	conn := dialAndAuth(t, token)
	defer conn.Close()

	// 2. 发送 ping 消息
	pingMsg := &msg.PingRequest{}
	msgID := msg_id.Ping
	err := writeProtobufMessage(conn, fw.MessageID(msgID), pingMsg)
	assert.NoError(t, err)
	logger.Info("✓ Ping message sent")

	// 3. 接收 pong 响应
	respData, respMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err)
	assert.Equal(t, fw.MessageID(msgID), respMsgID, "响应消息 ID 不匹配")

	var pongResp msg.PingResponse
	err = proto.Unmarshal(respData, &pongResp)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), pongResp.Code, "ping 失败，错误码: %d", pongResp.Code)
	logger.Info("✓ Pong response received")
}

// ========== 辅助函数 ==========

// dialAndAuth 连接 WebSocket 并完成认证
func dialAndAuth(t *testing.T, token string) *websocket.Conn {
	// 连接 WebSocket
	wsURL := "wss://" + testServerAddr + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v\n提示：请确保服务器正在运行并且 WebSocket 路由已正确配置", err)
	}

	// 发送认证消息（Protobuf 格式）
	authMsg := &msg.LoginAuthRequest{
		Token: token,
	}
	msgID := msg_id.LoginAuth
	err = writeProtobufMessage(conn, fw.MessageID(msgID), authMsg)
	assert.NoError(t, err, "发送认证消息失败")

	// 接收认证响应
	respData, respMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err, "读取认证响应失败")
	assert.Equal(t, fw.MessageID(msgID), respMsgID, "响应消息 ID 不匹配")

	var authResp msg.LoginAuthResponse
	err = proto.Unmarshal(respData, &authResp)
	assert.NoError(t, err, "解析认证响应失败")
	assert.Equal(t, int32(0), authResp.Code, "认证失败，错误码: %d", authResp.Code)

	return conn
}

// writeProtobufMessage 写入 Protobuf 消息（消息头 + 数据）
func writeProtobufMessage(conn *websocket.Conn, msgID fw.MessageID, msg proto.Message) error {
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
func readProtobufMessage(conn *websocket.Conn) ([]byte, fw.MessageID, error) {
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
	msgID := fw.MessageID(binary.BigEndian.Uint32(msgBytes[:4]))

	// 提取 protobuf 数据（4 字节之后）
	protoData := msgBytes[4:]

	return protoData, msgID, nil
}

// ---------------------------------------------------------
// TestWebSocketPing 测试 WebSocket Ping/Pong
func TestWatermelon(t *testing.T) {
	t.Log("server_addr: ", testServerAddr)
	// 1. 获取 token 并连接
	token := getTestToken(t)
	conn := dialAndAuth(t, token)
	defer conn.Close()

	{
		resp := &msg.WatermelonEndResponse{}
		testRPC(t, conn, msg_id.WatermelonEnd, &msg.WatermelonEndRequest{}, resp)
		assert.Equal(t, int32(0), resp.ErrorCode, "错误码: %d", resp.ErrorCode)
	}

	//开始获取掉落列表
	starResp := &msg.WatermelonStartResponse{}
	testRPC(t, conn, msg_id.WatermelonStart, &msg.WatermelonStartRequest{}, starResp)
	assert.Equal(t, int32(0), starResp.ErrorCode, "错误码: %d", starResp.ErrorCode)
	logger.Info("✓ response: ", starResp.String())
	if len(starResp.EntityLst) <= 0 {
		t.Fatal("err")
		return
	}

	starResp.Snapshot.Records = append(starResp.Snapshot.Records, starResp.EntityLst[0])

	//请求掉落1
	reqFall := &msg.WatermelonFallRequest{
		Snapshot:     starResp.Snapshot,
		WatermelonId: starResp.EntityLst[0].Id,
	}
	respFall := &msg.WatermelonFallResponse{}
	fmt.Println("cur1 record: ", reqFall.Snapshot.Records)
	testRPC(t, conn, msg_id.WatermelonFall, reqFall, respFall)
	assert.Equal(t, int32(0), respFall.ErrorCode, "错误码: %d", respFall.ErrorCode)
	logger.Info("✓ response: ", respFall.String())

	//请求掉落2
	starResp.Snapshot.Records = append(starResp.Snapshot.Records, respFall.EntityLst[0])
	reqFall2 := &msg.WatermelonFallRequest{
		Snapshot:     starResp.Snapshot,
		WatermelonId: respFall.EntityLst[0].Id,
	}
	fmt.Println("cur2 record: ", reqFall2.Snapshot.Records)
	testRPC(t, conn, msg_id.WatermelonFall, reqFall2, respFall)
	assert.Equal(t, int32(0), respFall.ErrorCode, "错误码: %d", respFall.ErrorCode)

	//合并1,2
	reqMerge := &msg.WatermelonSyncRequest{
		Snapshot: reqFall2.Snapshot,
	}
	reqMerge.Snapshot.Records = reqMerge.Snapshot.Records[1:]
	reqMerge.MergeLst = append(reqMerge.MergeLst, &msg.WatermelonMergeDetail{
		FromId: 1,
		ToId:   2,
	})
	respMerge := &msg.WatermelonSyncResponse{}
	testRPC(t, conn, msg_id.WatermelonSync, reqMerge, respMerge)
	assert.Equal(t, int32(0), respMerge.ErrorCode, "错误码: %d", respMerge.ErrorCode)
	logger.Info("✓ response: ", respMerge.String())

	pingMsg := &msg.PingRequest{}
	resp := &msg.PingResponse{}

	for i := 0; i < 1; i++ {
		beginTm := time.Now()
		testRPC(t, conn, msg_id.Ping, pingMsg, resp)
		if resp.Code != 0 {
			t.Fatal(resp)
		}
		fmt.Printf("cost[%d ms]\n", time.Since(beginTm).Milliseconds())
	}

	fmt.Println(".............")
	beginTm := time.Now()
	for i := 0; i < 1; i++ {
		onlySendRPC(t, conn, msg_id.Ping, pingMsg)
	}
	for i := 0; i < 1; i++ {
		onlyRevRPC(t, conn, msg_id.Ping, resp)
	}
	fmt.Printf("cost[%d ms]\n", time.Since(beginTm).Milliseconds())

	{
		req := &msg.RoleAlterNameRequest{
			Name: "test1",
		}
		respAlterName := &msg.RoleAlterNameResponse{}
		testRPC(t, conn, msg_id.RoleAlterName, req, respAlterName)
		assert.Equal(t, int32(0), respAlterName.Code, "错误码: %d", respAlterName.Code)

		req2 := &msg.RoleAlterFaceRequest{
			AvatarId: 0,
		}
		respAlterFace := &msg.RoleAlterFaceResponse{}
		testRPC(t, conn, msg_id.RoleAlterFace, req2, respAlterFace)
		assert.Equal(t, int32(0), respAlterFace.Code, "错误码: %d", respAlterFace.Code)
	}
}
func TestWatermelonEnd(t *testing.T) {
	// 1. 获取 token 并连接
	token := getTestToken(t)
	conn := dialAndAuth(t, token)
	defer conn.Close()

	// 2. 发送 ping 消息
	starResp := &msg.WatermelonEndResponse{}
	testRPC(t, conn, msg_id.WatermelonEnd, &msg.WatermelonEndRequest{}, starResp)
	assert.Equal(t, int32(0), starResp.ErrorCode, "错误码: %d", starResp.ErrorCode)
	logger.Info("✓ response: ", starResp.String())
}

func testRPC(t *testing.T, conn *websocket.Conn, msgId int32, req proto.Message, resp proto.Message) {
	err := writeProtobufMessage(conn, fw.MessageID(msgId), req)
	assert.NoError(t, err)

	//接收 响应
	respData, respMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err)
	assert.Equal(t, fw.MessageID(msgId), respMsgID, "响应消息 ID 不匹配")

	err = proto.Unmarshal(respData, resp)
	assert.NoError(t, err)
}

func onlySendRPC(t *testing.T, conn *websocket.Conn, msgId int32, req proto.Message) {
	err := writeProtobufMessage(conn, fw.MessageID(msgId), req)
	assert.NoError(t, err)
}

func onlyRevRPC(t *testing.T, conn *websocket.Conn, msgId int32, resp proto.Message) {
	//接收 响应
	respData, respMsgID, err := readProtobufMessage(conn)
	assert.NoError(t, err)
	assert.Equal(t, fw.MessageID(msgId), respMsgID, "响应消息 ID 不匹配")

	err = proto.Unmarshal(respData, resp)
	assert.NoError(t, err)
}

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
		"https://"+testServerAddr+"/api/login",
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
