package main

import (
	"fmt"
	"log"
	"time"

	"libx.net/workerid"
)

func main() {
	// 创建MemoryGenerator
	generator := workerid.NewMemoryGenerator(
		workerid.WithMaxWorkerID(10),           // 最大10个worker
		workerid.WithMaxLeaseTime(time.Minute), // 1分钟租约
	)

	// 示例1: 获取多个worker ID
	fmt.Println("🚀 Getting multiple worker IDs:")
	for i := 0; i < 5; i++ {
		workerID, token, err := generator.GetID()
		if err != nil {
			log.Fatalf("Failed to get worker ID: %v", err)
		}
		fmt.Printf("  Worker %d: ID=%d, Token=%s\n", i+1, workerID, token)

		// 模拟短暂使用
		time.Sleep(100 * time.Millisecond)
	}

	// 示例2: 获取特定worker ID并操作
	fmt.Println("\n🎯 Testing specific worker ID operations:")
	workerID, token, err := generator.GetID()
	if err != nil {
		log.Fatalf("Failed to get worker ID: %v", err)
	}

	fmt.Printf("Acquired worker ID: %d, Token: %s\n", workerID, token)

	// 续约测试
	fmt.Println("\n⏰ Testing renewal:")
	if err := generator.Renew(workerID, token); err != nil {
		log.Printf("Renew failed: %v", err)
	} else {
		fmt.Println("✅ Renewal successful")
	}

	// 错误测试: 使用错误的token
	fmt.Println("\n❌ Testing error cases:")
	wrongToken := "invalid_token_1234567890"
	if err := generator.Renew(workerID, wrongToken); err != nil {
		fmt.Printf("Expected error with wrong token: %v\n", err)
	} else {
		fmt.Println("Unexpected: renewal with wrong token succeeded")
	}

	// 释放worker ID
	fmt.Println("\n🗑️ Releasing worker ID:")
	if err := generator.Release(workerID, token); err != nil {
		log.Printf("Release failed: %v", err)
	} else {
		fmt.Println("✅ Release successful")
	}

	// 测试释放后再次获取
	fmt.Println("\n🔄 Testing re-acquisition after release:")
	newWorkerID, newToken, err := generator.GetID()
	if err != nil {
		log.Fatalf("Failed to get worker ID after release: %v", err)
	}
	fmt.Printf("Re-acquired worker ID: %d, Token: %s\n", newWorkerID, newToken)

	// 清理
	if err := generator.Release(newWorkerID, newToken); err != nil {
		log.Printf("Final release failed: %v", err)
	}

	fmt.Println("\n🎉 All tests completed successfully!")
}
