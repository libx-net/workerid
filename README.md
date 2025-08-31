# libx.net/workerid

A Go library for worker ID allocation and management in distributed systems.

## Features

- **Distributed Safety**: Supports worker ID allocation in distributed environments
- **Heartbeat Mechanism**: Supports worker liveliness detection
- **Easy to Use**: Clean API design
- **Multiple Storage Backends**: Default memory storage and Redis storage, supports custom storage

## Installation

```bash
go get libx.net/workerid
```

## Quick Start

### Using Redis Storage

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
        "mycluster", // Cluster name for distinguishing different clusters
        workerid.WithMaxWorkerID(32), // Optional, maximum worker count, default 1000
        workerid.WithMaxLeaseTime(time.Minute*5), // Optional, lease time, default 5 minutes
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
    }()
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
    }()
}
```

### Using Memory Storage

```go
package main

import (
    "fmt"
    "log"
    "time"

    "libx.net/workerid"
)

func main() {
    generator := workerid.NewMemoryGenerator(
        workerid.WithMaxWorkerID(100), // Optional, maximum worker count, default 1000
        workerid.WithMaxLeaseTime(time.Minute*2), // Optional, lease time, default 5 minutes
    )
    
    workerID, token, err := generator.GetID()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Acquired worker ID: %d, Token: %s\n", workerID, token)
    
    // Renew the lease
    if err := generator.Renew(workerID, token); err != nil {
        log.Printf("Renew failed: %v\n", err)
    }
    
    // Release the worker ID
    if err := generator.Release(workerID, token); err != nil {
        log.Printf("Release failed: %v\n", err)
    }
}
```

## API Reference

### Generator Interface

```go
type Generator interface {
    // GetID acquires a worker ID, returns worker ID and token (22-character string)
    GetID() (int64, string, error)
    // Release releases a worker ID
    Release(workerID int64, token string) error
    // Renew renews the lease of a worker ID
    Renew(workerID int64, token string) error
}
```

### RedisGenerator

Distributed worker ID allocator based on Redis.

```go
func NewRedisGenerator(client *redis.Client, cluster string, opts ...Option) (*RedisGenerator, error)
```

### MemoryGenerator

In-memory worker ID allocator, suitable for testing or single-node environments.

```go
func NewMemoryGenerator(opts ...Option) *MemoryGenerator
```

### Options

```go
// WithMaxWorkerID sets the maximum number of worker IDs
func WithMaxWorkerID(maxWorkers uint32) Option

// WithMaxLeaseTime sets the maximum lease duration
func WithMaxLeaseTime(maxLeaseTime time.Duration) Option
```

## Error Types

```go
var (
    ErrNoAvailableID   = errors.New("no available worker IDs")
    ErrInvalidWorkerID = errors.New("invalid worker ID")
    ErrTokenMismatch   = errors.New("token mismatch")
    ErrTokenExpired    = errors.New("token expired")
    ErrNotAssigned     = errors.New("worker ID not assigned")
    ErrInvalidToken    = errors.New("invalid token format")
)
```

## Performance Considerations

- **Redis**: Suitable for high-concurrency scenarios, supports distributed deployment, uses distributed locks to prevent two workers from acquiring the same worker ID simultaneously. Recommended maximum worker count is 1000.
- **Memory**: Highest performance, suitable for testing environments.

## Implementation Details

- **Token Format**: 22-character base64 URL-encoded random string
- **Redis Implementation**: Uses Lua scripts for atomic operations and Redis sorted sets for ID management
- **Memory Implementation**: Uses mutex locks for thread safety

## Examples

Check the `examples/` directory for complete usage examples:

- `redis/`: Redis-based worker ID management with renewal and graceful shutdown
- `memory/`: Memory-based usage examples with error handling and testing scenarios

To run the examples:
```bash
# Redis example
cd examples/redis
go mod tidy
go run main.go

# Memory example  
cd examples/memory
go mod tidy
go run main.go
```

## License

MIT License

## Contributing

Welcome to submit Issues and Pull Requests!