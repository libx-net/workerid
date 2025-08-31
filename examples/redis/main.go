package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"libx.net/workerid"
)

func main() {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis服务器地址
		Password: "",               // 密码
		DB:       0,                // 数据库编号
	})

	// 测试Redis连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// 创建RedisGenerator
	generator, err := workerid.NewRedisGenerator(
		client,
		"my-app-cluster",             // 集群名称
		workerid.WithMaxWorkerID(50), // 最大50个worker
		workerid.WithMaxLeaseTime(2*time.Minute), // 2分钟租约
	)
	if err != nil {
		log.Fatalf("Failed to create RedisGenerator: %v", err)
	}

	// 获取worker ID
	workerID, token, err := generator.GetID()
	if err != nil {
		log.Fatalf("Failed to get worker ID: %v", err)
	}

	fmt.Printf("✅ Acquired worker ID: %d\n", workerID)
	fmt.Printf("🔑 Token: %s\n", token)

	// 设置信号处理，优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建续约定时器
	renewTicker := time.NewTicker(30 * time.Second) // 每30秒续约一次
	defer renewTicker.Stop()

	// 主循环
	running := true
	for running {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Received shutdown signal")
			running = false

		case <-renewTicker.C:
			// 续约worker ID
			if err := generator.Renew(workerID, token); err != nil {
				log.Printf("⚠️ Failed to renew worker ID: %v", err)
				running = false
			} else {
				fmt.Printf("🔄 Worker ID %d renewed successfully\n", workerID)
			}
		}
	}

	// 释放worker ID
	if err := generator.Release(workerID, token); err != nil {
		log.Printf("⚠️ Failed to release worker ID: %v", err)
	} else {
		fmt.Printf("✅ Worker ID %d released successfully\n", workerID)
	}

	fmt.Println("👋 Application exited")
}
