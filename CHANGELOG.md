# WorkerID ChangeLog

## WorkerID v0.3.0

Major breaking release relative to v0.2.0: the core library has **zero third-party dependencies**, Redis and SQL backends are driver/ORM-agnostic, and official adapter submodules are provided for popular clients.

### ⚠️ BREAKING CHANGES

* **feat!: make Redis generator driver-agnostic; drop hard dependency on go-redis v8**
  - `NewRedisGenerator` now takes `RedisDoer` instead of `*redis.Client` from `github.com/go-redis/redis/v8`
  - Root module has no third-party `require` entries (standard library only, including `database/sql`)
  - Call sites that passed `*redis.Client` must migrate (see migration guide below)

* **feat!: unify minimum Go version to 1.22**
  - Root module and all adapter / example submodules declare `go 1.22` (or equivalent `go 1.22.0`)

### Migration guide (from v0.2.x)

**Option A: official Redis adapters (recommended)**

```go
import (
    "github.com/redis/go-redis/v9" // or github.com/go-redis/redis/v8
    goredisv9 "libx.net/workerid/adapter/goredis-v9"
    "libx.net/workerid"
)

client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
gen, err := goredisv9.NewGenerator(client, "cluster",
    workerid.WithWorkerBits(8),
    workerid.WithMaxLeaseTime(5*time.Minute),
)
```

```bash
go get libx.net/workerid/adapter/goredis-v9
# or
go get libx.net/workerid/adapter/goredis-v8
```

**Option B: adapt RedisDoer yourself**

```go
// before (v0.2.x)
gen, err := workerid.NewRedisGenerator(client, "cluster", opts...)

// after (v0.3.0) — go-redis v8 / v9
doer := workerid.RedisFunc(func(ctx context.Context, args ...any) (any, error) {
    return client.Do(ctx, args...).Result()
})
gen, err := workerid.NewRedisGenerator(doer, "cluster", opts...)

// go-redis v7 (Do has no ctx)
doer := workerid.RedisFunc(func(_ context.Context, args ...any) (any, error) {
    return client.Do(args...).Result()
})
```

**SQL backends** are new in v0.3.0; see the README for `database/sql`, GORM, sqlx, and Bun usage. Nothing to migrate.

**MemoryGenerator / Option API**: no breaking changes relative to v0.2.0 (`WithWorkerBits` / `WithMaxLeaseTime` unchanged).

### Features

* feat: add `RedisDoer` interface and `RedisFunc` adapter type
* feat: official Redis adapter submodules
  - `libx.net/workerid/adapter/goredis-v8` (`github.com/go-redis/redis/v8`)
  - `libx.net/workerid/adapter/goredis-v9` (`github.com/redis/go-redis/v9`)
  - expose `NewDoer` / `NewGenerator` accepting `redis.UniversalClient`
* feat: add root SQL support with zero third-party dependencies
  - `SQLClient` / `SQLTx` / `SQLRow` / `SQLDialect`
  - `NewDatabaseSQLClient`, `NewSQLGenerator`, `InitializeSQLCluster`
  - dialects for PostgreSQL, MySQL 5.7, MySQL 8.0+, and SQLite (user-managed schema; no auto DDL)
* feat: official SQL ORM adapter submodules
  - `libx.net/workerid/adapter/gorm`
  - `libx.net/workerid/adapter/sqlx`
  - `libx.net/workerid/adapter/bun`
  - each depends only on the root module plus its named ORM (no DB drivers)
* feat: export shared `GeneratorConfig` and `ResolveConfig`
* feat: Redis `GetID` returns `ErrNoAvailableID` when no IDs are available
* feat: Redis `initAvailableIDs` / `Release` are atomic Lua scripts; `Renew` / `Release` use unified status-string error mapping

### Bug fixes

* fix: MySQL `workerid_leases.expire_at` is `DATETIME(6)` so `MySQLSchema` applies under `STRICT_TRANS_TABLES,NO_ZERO_DATE` (TIMESTAMP Unix-epoch default is Error 1067). `CREATE TABLE IF NOT EXISTS` does not migrate an existing TIMESTAMP column — run the `ALTER TABLE ... MODIFY expire_at DATETIME(6)` in `schema_mysql.sql` / README yourself (no auto-DDL)
* fix: `Renew` correctly maps NOT_FOUND / MISMATCH / EXPIRED (old `{err=...}` Lua replies were not matched as result strings under go-redis)

### Documentation updates

* docs: rewrite README for zero-dependency core, Redis/SQL abstractions, built-in dialects, and ORM adapters
* docs: fix examples/redis (`WithMaxWorkerID` → `WithWorkerBits`, use `RedisDoer` adapter)

## WorkerID v0.2.0

### ⚠️ BREAKING CHANGES
* **feat!: replace WithMaxWorkerID with WithWorkerBits option** (387378c) (@krwu)
  - Replace `WithMaxWorkerID(maxWorkers uint32)` with `WithWorkerBits(workerBits uint)`
  - Change default maxWorkerID from 1000 to 511 (9 bits)
  - Update WorkerID range to start from 0 instead of 1
  - Migration guide: Replace `WithMaxWorkerID(n)` with `WithWorkerBits(bits)` where `n = (1<<bits)-1`

### Features
* feat: add comprehensive TestWithWorkerBits test covering multiple bit configurations (387378c) (@krwu)
* feat: ensure Redis initialization creates WorkerIDs from 0 to maxWorkerID (387378c) (@krwu)

### Documentation updates
* docs: fix parameter type inconsistency in documentation (uint32 -> uint) (387378c) (@krwu)
* docs: remove Chinese README (README_CN.md) to maintain single documentation (387378c) (@krwu)

### Testing improvements
* test: update all test cases to use new WithWorkerBits option (387378c) (@krwu)

## WorkerID v0.1.3

### Bug fixes
* fix: resolve hash expiration issue in renew operation (e35d62f) (@krwu)

### Testing improvements
* test: add comprehensive test for hash expiration behavior during renew operations (e35d62f) (@krwu)

## WorkerID v0.1.2

### Refactoring
* refactor: rewrite Renew with Lua instead of transactions for Redis clusters that do not support MULTI/EXEC (762c4dd) (@krwu)

### Testing improvements
* test: add comprehensive unit tests with miniredis framework (ec6cbf9) (@krwu)

## WorkerID v0.1.1

### Features
* feat: add Redis and memory worker ID generators

## WorkerID v0.1.0

Initial release (retracted).
