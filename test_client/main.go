package main

import (
	"fmt"
)

func main() {
	err := ExampleWatermelon()
	if err != nil {
		fmt.Printf("示例运行失败: %v\n", err)
	}
}
