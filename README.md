# libx.net/workerid

A Go library for worker ID allocation and management in distributed systems.

## Features

- **Distributed Safety**: Supports worker ID allocation in distributed environments
- **Heartbeat Mechanism**: Supports worker liveliness detection
- **Easy to Use**: Clean API design
- **Multiple Storage Backends**: Memory, Redis, and SQL (PostgreSQL / MySQL / SQLite)
- **Zero third-party dependencies in the core module**: only the Go standard library (including `database/sql`)
- **Redis Driver Agnostic**: Works with go-redis v7/v8/v9 (or any client) via `RedisDoer`
- **SQL Client Agnostic**: Works with `database/sql`, GORM, sqlx, or Bun via `SQLClient` / `SQLTx`

## Installation

```bash
go get libx.net/workerid
```

Official adapters (optional):

```bash
go get libx.net/workerid/adapter/goredis-v8
go get libx.net/workerid/adapter/goredis-v9
go get libx.net/workerid/adapter/gorm
go get libx.net/workerid/adapter/sqlx
go get libx.net/workerid/adapter/bun
```

## Quick Start

### Using Official go-redis Adapter (recommended)

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/redis/go-redis/v9"
    goredisv9 "libx.net/workerid/adapter/goredis-v9"
    "libx.net/workerid"
)

func main() {
    client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

    generator, err := goredisv9.NewGenerator(
        client,
        "mycluster",
        workerid.WithWorkerBits(8),
        workerid.WithMaxLeaseTime(5*time.Minute),
    )
    if err != nil {
        log.Fatal(err)
    }

    workerID, token, err := generator.GetID()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Acquired worker ID: %d, token: %s\n", workerID, token)
}
```

For go-redis v8, use `libx.net/workerid/adapter/goredis-v8` the same way
(`github.com/go-redis/redis/v8`).

### Using Redis Storage (manual RedisDoer)

`NewRedisGenerator` accepts a `RedisDoer` instead of a concrete go-redis client.
Adapt your preferred go-redis version with a small closure:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "libx.net/workerid"
    "github.com/go-redis/redis/v8" // or github.com/redis/go-redis/v9
)

func main() {
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // go-redis v8 / v9
    doer := workerid.RedisFunc(func(ctx context.Context, args ...any) (any, error) {
        return client.Do(ctx, args...).Result()
    })
    // go-redis v7 (Do has no ctx):
    // doer := workerid.RedisFunc(func(_ context.Context, args ...any) (any, error) {
    //     return client.Do(args...).Result()
    // })

    generator, err := workerid.NewRedisGenerator(
        doer,
        "mycluster",
        workerid.WithWorkerBits(8),
        workerid.WithMaxLeaseTime(time.Minute*5),
    )
    if err != nil {
        log.Fatal(err)
    }
    workerID, token, err := generator.GetID()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Acquired worker ID: %d\n", workerID)
    _ = token
}
```

### Using SQL Storage (PostgreSQL / MySQL / SQLite)

SQL support lives in the **root module**. You choose:

1. A **client** (`database/sql`, GORM, sqlx, or Bun) via `SQLClient`
2. A **dialect** for the database product (`PostgresDialect`, `MySQL57Dialect`, `MySQL80Dialect`, `SQLiteDialect`)

You must:

1. Apply the matching schema (`PostgresSchema` / `MySQLSchema` / `SQLiteSchema`, also shipped as `schema_*.sql`) with your migration tool (the library does **not** auto-DDL).
2. Call `InitializeSQLCluster` once to seed the ID pool.
3. Create a generator with `NewSQLGenerator`.

Supported dialects:

| Dialect | Constructor | Concurrency notes |
|---|---|---|
| PostgreSQL | `PostgresDialect()` | `FOR UPDATE SKIP LOCKED` |
| MySQL 8.0+ | `MySQL80Dialect()` | `FOR UPDATE SKIP LOCKED` |
| MySQL 5.7 | `MySQL57Dialect()` | `FOR UPDATE` (blocking) |
| SQLite | `SQLiteDialect()` | Single-writer reserved lock; limited throughput |

#### database/sql (stdlib wrapper)

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
    "libx.net/workerid"
)

func main() {
    db, err := sql.Open("pgx", "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 1) Apply workerid.PostgresSchema via your migration system first.
    client := workerid.NewDatabaseSQLClient(db)
    dialect := workerid.PostgresDialect()
    opts := []workerid.Option{
        workerid.WithWorkerBits(8),
        workerid.WithMaxLeaseTime(5 * time.Minute),
    }

    // 2) Seed the cluster once during deploy/bootstrap:
    if err := workerid.InitializeSQLCluster(context.Background(), client, dialect, "mycluster", opts...); err != nil {
        log.Fatal(err)
    }

    generator, err := workerid.NewSQLGenerator(client, dialect, "mycluster", opts...)
    if err != nil {
        log.Fatal(err)
    }

    workerID, token, err := generator.GetID()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Acquired worker ID: %d, token: %s\n", workerID, token)
}
```

#### GORM / sqlx / Bun adapters

```go
import (
    gormadapter "libx.net/workerid/adapter/gorm"
    // sqlxadapter "libx.net/workerid/adapter/sqlx"
    // bunadapter "libx.net/workerid/adapter/bun"
)

// After applying schema:
_ = gormadapter.InitializeCluster(ctx, gdb, workerid.PostgresDialect(), "mycluster", opts...)
gen, err := gormadapter.NewGenerator(gdb, workerid.PostgresDialect(), "mycluster", opts...)
```

Each ORM adapter exposes `NewClient`, `NewGenerator`, and `InitializeCluster`, and depends only on the root module plus its named ORM — never on a concrete database driver.

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
        workerid.WithWorkerBits(9),
        workerid.WithMaxLeaseTime(time.Minute*2),
    )

    workerID, token, err := generator.GetID()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Acquired worker ID: %d, Token: %s\n", workerID, token)

    if err := generator.Renew(workerID, token); err != nil {
        log.Printf("Renew failed: %v\n", err)
    }

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

### RedisDoer

Driver-agnostic Redis command executor used by `RedisGenerator`.

```go
type RedisDoer interface {
    Do(ctx context.Context, args ...any) (any, error)
}

// RedisFunc lets any function implement RedisDoer
type RedisFunc func(ctx context.Context, args ...any) (any, error)
```

### RedisGenerator

Distributed worker ID allocator based on Redis.

```go
func NewRedisGenerator(client RedisDoer, cluster string, opts ...Option) (*RedisGenerator, error)
```

### Official Redis Adapters

```go
// adapter/goredis-v8
func NewDoer(client redis.UniversalClient) workerid.RedisDoer
func NewGenerator(client redis.UniversalClient, cluster string, options ...workerid.Option) (*workerid.RedisGenerator, error)

// adapter/goredis-v9
func NewDoer(client redis.UniversalClient) workerid.RedisDoer
func NewGenerator(client redis.UniversalClient, cluster string, options ...workerid.Option) (*workerid.RedisGenerator, error)
```

### SQLClient / SQLGenerator

```go
type SQLRow interface {
    Scan(dest ...any) error
}

type SQLTx interface {
    QueryRow(ctx context.Context, query string, args ...any) SQLRow
    Exec(ctx context.Context, query string, args ...any) error
    Commit() error
    Rollback() error
}

type SQLClient interface {
    BeginTx(ctx context.Context) (SQLTx, error)
}

func NewDatabaseSQLClient(db *sql.DB) SQLClient
func NewSQLGenerator(client SQLClient, dialect SQLDialect, cluster string, options ...Option) (*SQLGenerator, error)
func InitializeSQLCluster(ctx context.Context, client SQLClient, dialect SQLDialect, cluster string, options ...Option) error

func PostgresDialect() SQLDialect
func MySQL57Dialect() SQLDialect
func MySQL80Dialect() SQLDialect
func SQLiteDialect() SQLDialect

var PostgresSchema string
var MySQLSchema string
var SQLiteSchema string
```

### Official SQL ORM Adapters

```go
// adapter/gorm, adapter/sqlx, adapter/bun
func NewClient(...) workerid.SQLClient
func NewGenerator(..., dialect workerid.SQLDialect, cluster string, options ...workerid.Option) (*workerid.SQLGenerator, error)
func InitializeCluster(ctx, ..., dialect workerid.SQLDialect, cluster string, options ...workerid.Option) error
```

### MemoryGenerator

In-memory worker ID allocator, suitable for testing or single-node environments.

```go
func NewMemoryGenerator(opts ...Option) *MemoryGenerator
```

### Options

```go
// WithWorkerBits sets bits for store workerID
func WithWorkerBits(workerBits uint) Option

// WithMaxLeaseTime sets the maximum lease duration
func WithMaxLeaseTime(maxLeaseTime time.Duration) Option

// ResolveConfig applies options with shared defaults
func ResolveConfig(cluster string, options ...Option) (GeneratorConfig, error)
```

## Error Types

```go
var (
    ErrNoAvailableID         = errors.New("no available worker IDs")
    ErrInvalidWorkerID       = errors.New("invalid worker ID")
    ErrTokenMismatch         = errors.New("token mismatch")
    ErrTokenExpired          = errors.New("token expired")
    ErrNotAssigned           = errors.New("worker ID not assigned")
    ErrInvalidToken          = errors.New("invalid token format")
    ErrClusterConfigMismatch = errors.New("cluster max_worker_id mismatch")
    ErrSQLNoRows             = errors.New("sql: no rows in result set")
)
```

## Performance Considerations

- **Redis**: Suitable for high-concurrency scenarios, supports distributed deployment, uses Lua scripts for atomic operations. Recommended workerBits is 10 (max 1023).
- **PostgreSQL / MySQL 8.0+**: Good for SQL-centric deployments; uses `FOR UPDATE SKIP LOCKED`.
- **MySQL 5.7**: Correct under concurrency but may block on row locks (no `SKIP LOCKED`).
- **SQLite**: Correct single-writer model; not intended for high allocation throughput.
- **Memory**: Highest performance, suitable for testing environments.

## Implementation Details

- **Token Format**: 22-character base64 URL-encoded random string
- **Redis Implementation**: Uses Lua scripts for atomic operations and Redis sorted sets for ID management; no hard dependency on a specific go-redis version
- **SQL Implementation**: Tables `workerid_clusters` and `workerid_leases`; schema is user-managed; lease time uses database-side timestamps; client and dialect are separate concerns
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
