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
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // Password
		DB:       0,                // Database number
	})

	// Test Redis connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Adapt go-redis client to RedisDoer
	doer := workerid.RedisFunc(func(ctx context.Context, args ...any) (any, error) {
		return client.Do(ctx, args...).Result()
	})

	// Create RedisGenerator
	generator, err := workerid.NewRedisGenerator(
		doer,
		"my-app-cluster",                       // Cluster name
		workerid.WithWorkerBits(6),             // maxWorkerID = 63
		workerid.WithMaxLeaseTime(2*time.Minute), // 2-minute lease
	)
	if err != nil {
		log.Fatalf("Failed to create RedisGenerator: %v", err)
	}

	// Acquire worker ID
	workerID, token, err := generator.GetID()
	if err != nil {
		log.Fatalf("Failed to get worker ID: %v", err)
	}

	fmt.Printf("✅ Acquired worker ID: %d\n", workerID)
	fmt.Printf("🔑 Token: %s\n", token)

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Renewal ticker
	renewTicker := time.NewTicker(30 * time.Second) // renew every 30 seconds
	defer renewTicker.Stop()

	// Main loop
	running := true
	for running {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Received shutdown signal")
			running = false

		case <-renewTicker.C:
			// Renew worker ID
			if err := generator.Renew(workerID, token); err != nil {
				log.Printf("⚠️ Failed to renew worker ID: %v", err)
				running = false
			} else {
				fmt.Printf("🔄 Worker ID %d renewed successfully\n", workerID)
			}
		}
	}

	// Release worker ID
	if err := generator.Release(workerID, token); err != nil {
		log.Printf("⚠️ Failed to release worker ID: %v", err)
	} else {
		fmt.Printf("✅ Worker ID %d released successfully\n", workerID)
	}

	fmt.Println("👋 Application exited")
}
