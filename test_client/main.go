package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"time"
	"wx_game/msg"
	"wx_game/msg/msg_id"
)

func main() {
	// 解析命令行参数
	count := flag.Int("count", 20, "Ping 的次数")
	serverAddr := flag.String("addr", "43.100.128.210:8080", "服务器地址")
	flag.Parse()

	// 创建客户端
	client := NewClient(*serverAddr)

	// 获取 token
	fmt.Println("正在获取 token...")
	err := client.GetToken()
	if err != nil {
		fmt.Printf("获取 token 失败: %v\n", err)
		waitExit()
		return
	}

	// 连接并认证
	fmt.Println("正在连接服务器...")
	err = client.Connect()
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		waitExit()
		return
	}
	defer client.Close()

	fmt.Printf("连接成功！开始 Ping %d 次...\n\n", *count)

	// 同步 Ping
	pingMsg := &msg.PingRequest{}
	resp := &msg.PingResponse{}

	successCount := 0
	failCount := 0
	totalTime := time.Duration(0)
	var minTime, maxTime time.Duration
	firstSuccess := true

	for i := 0; i < *count; i++ {
		beginTm := time.Now()
		err = client.CallRPC(msg_id.Ping, pingMsg, resp)
		cost := time.Since(beginTm)

		if err != nil {
			fmt.Printf("[%d/%d] Ping 失败: %v\n", i+1, *count, err)
			failCount++
		} else if resp.Code != 0 {
			fmt.Printf("[%d/%d] Ping 错误，错误码: %d\n", i+1, *count, resp.Code)
			failCount++
		} else {
			fmt.Printf("[%d/%d] Ping 成功，耗时: %d ms\n", i+1, *count, cost.Milliseconds())
			successCount++
			totalTime += cost

			// 更新最高和最低延迟
			if firstSuccess {
				minTime = cost
				maxTime = cost
				firstSuccess = false
			} else {
				if cost < minTime {
					minTime = cost
				}
				if cost > maxTime {
					maxTime = cost
				}
			}
		}
	}

	// 统计信息
	fmt.Println("\n========== 统计信息 ==========")
	fmt.Printf("总次数: %d\n", *count)
	fmt.Printf("成功: %d\n", successCount)
	fmt.Printf("失败: %d\n", failCount)
	if successCount > 0 {
		avgTime := totalTime / time.Duration(successCount)
		fmt.Printf("平均耗时: %d ms\n", avgTime.Milliseconds())
		fmt.Printf("最低延迟: %d ms\n", minTime.Milliseconds())
		fmt.Printf("最高延迟: %d ms\n", maxTime.Milliseconds())
		fmt.Printf("总耗时: %d ms\n", totalTime.Milliseconds())
	}
	fmt.Println("==============================")
}

func waitExit() {
	fmt.Println("\n按回车键退出...")
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadByte()
}
