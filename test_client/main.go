package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	err := ExampleWatermelon()
	if err != nil {
		fmt.Printf("示例运行失败: %v\n", err)
	}

	// 按任意键退出
	fmt.Println("\n按回车键退出...")
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadByte()
}
