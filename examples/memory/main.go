package main

import (
	"fmt"
	"log"

	"libx.net/workerid"
)

func main() {
	// 创建MemoryGenerator
	generator := workerid.NewMemoryGenerator()

	// 获取worker ID和token
	workerID, token, err := generator.GetID()
	if err != nil {
		log.Fatalf("Failed to get worker ID: %v", err)
	}

	fmt.Printf("Acquired worker ID: %d, Token: %s\n", workerID, token)

	// 测试续约功能
	fmt.Println("\nTesting renewal with correct token:")
	if err := generator.Renew(workerID, token); err != nil {
		log.Printf("Renew failed: %v", err)
	} else {
		fmt.Println("✅ Renewal successful")
	}

	// 测试错误token
	fmt.Println("\nTesting renewal with wrong token:")
	wrongToken := "invalid_token_1234567890"
	if err := generator.Renew(workerID, wrongToken); err != nil {
		fmt.Printf("Expected error with wrong token: %v\n", err)
	} else {
		fmt.Println("Unexpected: renewal with wrong token succeeded")
	}

	// 测试释放功能
	fmt.Println("\nTesting release with correct token:")
	if err := generator.Release(workerID, token); err != nil {
		log.Printf("Release failed: %v", err)
	} else {
		fmt.Println("✅ Release successful")
	}

	fmt.Println("\n🎉 MemoryGenerator example completed!")
}
