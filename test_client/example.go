package main

import (
	"fmt"
	"time"
	"wx_game/msg"
	"wx_game/msg/msg_id"

	"github.com/donnie4w/go-logger/logger"
)

// ExampleWatermelon 示例：西瓜游戏完整流程
// 参考 main_test.go 中的 TestWatermelon 函数
func ExampleWatermelon() error {
	// 测试服务器地址
	serverAddr := "43.100.128.210:8080"
	// serverAddr := "127.0.0.1:8080"

	// 1. 创建客户端
	client := NewClient(serverAddr)

	// 2. 获取 token
	err := client.GetToken()
	if err != nil {
		return fmt.Errorf("获取 token 失败: %w", err)
	}

	// 3. 连接并认证
	err = client.Connect()
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer client.Close()

	// 4. 结束上一局游戏
	{
		resp := &msg.WatermelonEndResponse{}
		err = client.CallRPC(msg_id.WatermelonEnd, &msg.WatermelonEndRequest{}, resp)
		if err != nil {
			return fmt.Errorf("结束游戏失败: %w", err)
		}
		if resp.ErrorCode != 0 {
			return fmt.Errorf("结束游戏错误，错误码: %d", resp.ErrorCode)
		}
	}

	// 5. 开始获取掉落列表
	starResp := &msg.WatermelonStartResponse{}
	err = client.CallRPC(msg_id.WatermelonStart, &msg.WatermelonStartRequest{}, starResp)
	if err != nil {
		return fmt.Errorf("开始游戏失败: %w", err)
	}
	if starResp.ErrorCode != 0 {
		return fmt.Errorf("开始游戏错误，错误码: %d", starResp.ErrorCode)
	}
	logger.Info("✓ response: ", starResp.String())
	if len(starResp.EntityLst) <= 0 {
		return fmt.Errorf("掉落列表为空")
	}

	starResp.Snapshot.Records = append(starResp.Snapshot.Records, starResp.EntityLst[0])

	// 6. 请求掉落1
	reqFall := &msg.WatermelonFallRequest{
		Snapshot:     starResp.Snapshot,
		WatermelonId: starResp.EntityLst[0].Id,
	}
	respFall := &msg.WatermelonFallResponse{}
	fmt.Println("cur1 record: ", reqFall.Snapshot.Records)
	err = client.CallRPC(msg_id.WatermelonFall, reqFall, respFall)
	if err != nil {
		return fmt.Errorf("掉落1失败: %w", err)
	}
	if respFall.ErrorCode != 0 {
		return fmt.Errorf("掉落1错误，错误码: %d", respFall.ErrorCode)
	}
	logger.Info("✓ response: ", respFall.String())

	// 7. 请求掉落2
	starResp.Snapshot.Records = append(starResp.Snapshot.Records, respFall.EntityLst[0])
	reqFall2 := &msg.WatermelonFallRequest{
		Snapshot:     starResp.Snapshot,
		WatermelonId: respFall.EntityLst[0].Id,
	}
	fmt.Println("cur2 record: ", reqFall2.Snapshot.Records)
	err = client.CallRPC(msg_id.WatermelonFall, reqFall2, respFall)
	if err != nil {
		return fmt.Errorf("掉落2失败: %w", err)
	}
	if respFall.ErrorCode != 0 {
		return fmt.Errorf("掉落2错误，错误码: %d", respFall.ErrorCode)
	}

	// 8. 合并1,2
	reqMerge := &msg.WatermelonSyncRequest{
		Snapshot: reqFall2.Snapshot,
	}
	reqMerge.Snapshot.Records = reqMerge.Snapshot.Records[1:]
	reqMerge.MergeLst = append(reqMerge.MergeLst, &msg.WatermelonMergeDetail{
		FromId: 1,
		ToId:   2,
	})
	respMerge := &msg.WatermelonSyncResponse{}
	err = client.CallRPC(msg_id.WatermelonSync, reqMerge, respMerge)
	if err != nil {
		return fmt.Errorf("合并失败: %w", err)
	}
	if respMerge.ErrorCode != 0 {
		return fmt.Errorf("合并错误，错误码: %d", respMerge.ErrorCode)
	}
	logger.Info("✓ response: ", respMerge.String())

	// 9. 测试 Ping（同步方式）
	pingMsg := &msg.PingRequest{}
	resp := &msg.PingResponse{}

	fmt.Println("测试同步 Ping...")
	for i := 0; i < 20; i++ {
		beginTm := time.Now()
		err = client.CallRPC(msg_id.Ping, pingMsg, resp)
		if err != nil {
			return fmt.Errorf("Ping 失败: %w", err)
		}
		if resp.Code != 0 {
			return fmt.Errorf("Ping 错误，错误码: %d", resp.Code)
		}
		fmt.Printf("cost[%d ms]\n", time.Since(beginTm).Milliseconds())
	}

	// 10. 测试 Ping（批量发送 + 批量接收）
	fmt.Println(".............")
	fmt.Println("测试批量 Ping...")
	beginTm := time.Now()
	for i := 0; i < 20; i++ {
		err = client.SendRPC(msg_id.Ping, pingMsg)
		if err != nil {
			return fmt.Errorf("批量发送 Ping 失败: %w", err)
		}
	}
	for i := 0; i < 20; i++ {
		err = client.ReceiveRPC(msg_id.Ping, resp)
		if err != nil {
			return fmt.Errorf("批量接收 Ping 失败: %w", err)
		}
	}
	fmt.Printf("cost[%d ms]\n", time.Since(beginTm).Milliseconds())

	return nil
}
