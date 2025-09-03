# WorkerID ChangeLog

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
* refactor: renew 方法改用 lua 代替事务，兼容某些不支持事务的 redis 集群 (762c4dd) (@krwu)

### Testing improvements
* test: add comprehensive unit tests with miniredis framework (ec6cbf9) (@krwu)

## WorkerID v0.1.1

### Bug fixes
* fix: fix the bug that the worker id is not unique in the same process (e5f8320) (@krwu)

### Others
* chore: add golangci config and update Go Modules (2a321af) (@krwu)

## WorkerID v0.1.0

### Features
* refactor: optimize Redis generator code structure (7099305) (@krwu)

### CI/CD improvements
* ci: setup lint tool and github workflow (ec3a40b) (@krwu)

### Features
* feat: simplify MemoryGenerator for single-worker environment (c85ba0b) (@krwu)
* feat: first implement of WorkerID library with memory and redis generators (a6ca7af) (@krwu)

### Documentation updates
* docs: add English README and update Chinese documentation (26b441d) (@krwu)

### Others
* chore: Add MIT LICENSE file (79a0355) (@krwu)
* chore: add examples (69c0706) (@krwu)