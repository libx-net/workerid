# libx.net/workerid

一个用于分布式系统中worker ID分配和管理的Go库。

## 特性

- **分布式安全**: 支持分布式环境下的worker ID分配
- **心跳机制**: 支持worker存活状态检测
- **易于使用**: 简洁的API设计
- **多存储支持**: 默认提供内存存储和 Redis 存储，支持自定义存储

## 安装

```bash
go get libx.net/workerid
```

## 快速开始

### 使用Redis存储

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"libx.net/workerid"
	"github.com/go-redis/redis/v8"
)

var (
    workerIDToken = ""
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

    generator, err := workerid.NewRedisGenerator(
        client,
        "mycluster", // 集群名称，用于区分不同集群
        workerid.WithMaxWorkerID(32), // 可选，最大worker数量，默认1000
        workerid.WithMaxLeaseTime(time.Minute*5), // 可选，租约时间，默认5分钟
    )
    if err != nil {
        log.Fatal(err)
    }
	workerID, token, err := generator.GetID()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Acquired worker ID: %d\n", workerID)
    ctx, cancel := context.WithCancel(context.Background())
    defer func() {
        if err := generator.Release(workerID, token); err != nil {
            log.Printf("[WARN] release worker ID failed: %v\n", err)
        }
        cancel()
    }
	ticker := time.NewTicker(time.Second * 90)
    go func() {
        select {
            case <- ctx.Done():
                fmt.Println("main process exit")
                ticker.Stop()
            case <-ticker.C:
                if err := generator.Renew(workerID, token); err != nil {
                    log.Fatal(err)
                    ticker.Stop()
                    return
                }
        }
    }
}
```

## API参考

### Generator接口

```go
type Generator interface {
    // GetID 获取worker ID,返回 worker ID 和 token
	GetID() (int64, string, error)
	Release(workerID int64, token string) error
    Renew(workerID int64, token string) error
}
```

### RedisGenerator

基于Redis的分布式worker ID分配器。

```go
func NewRedisGenerator(client *redis.Client, cluster string, opts ...Option) *RedisGenerator
```

### MemoryGenerator

基于内存的worker ID分配器，适用于测试或单机环境。

```go
func NewMemoryGenerator(opts ...Option) *MemoryGenerator
```

## 性能考虑

- **Redis**: 适合高并发场景，支持分布式部署，采用分布式锁避免两个 worker 同时获取同一个 worker ID，建议最大worker数量不超过1000
- **内存**: 最高性能，适合测试环境

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！