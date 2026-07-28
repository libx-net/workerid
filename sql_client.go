package workerid

import (
	"context"
	"errors"
)

// ErrClusterConfigMismatch is returned when InitializeSQLCluster finds an existing
// cluster whose max_worker_id differs from the configured value.
var ErrClusterConfigMismatch = errors.New("cluster max_worker_id mismatch")

// ErrSQLNoRows is returned when a QueryRow finds no matching row.
// NewDatabaseSQLClient maps database/sql.ErrNoRows to this error.
var ErrSQLNoRows = errors.New("sql: no rows in result set")

// SQLRow is a single-row query result.
type SQLRow interface {
	Scan(dest ...any) error
}

// SQLTx is a driver/ORM-agnostic database transaction.
type SQLTx interface {
	QueryRow(ctx context.Context, query string, args ...any) SQLRow
	Exec(ctx context.Context, query string, args ...any) error
	Commit() error
	Rollback() error
}

// SQLClient begins transactions for SQLGenerator.
type SQLClient interface {
	BeginTx(ctx context.Context) (SQLTx, error)
}

// SQLDialect encapsulates database-product-specific SQL and transaction setup.
type SQLDialect interface {
	// PrepareTx runs after BeginTx. Most dialects no-op; SQLite escalates to a reserved lock.
	PrepareTx(ctx context.Context, tx SQLTx) error

	LookupClusterMax(ctx context.Context, tx SQLTx, cluster string) (max uint32, ok bool, err error)
	InsertCluster(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error
	SeedLeases(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error

	SelectAvailable(ctx context.Context, tx SQLTx, cluster string) (workerID int64, err error)
	Claim(ctx context.Context, tx SQLTx, cluster string, workerID int64, token string, leaseSeconds int) error
	SelectLease(ctx context.Context, tx SQLTx, cluster string, workerID int64) (token string, expireAtUnix int64, err error)
	Now(ctx context.Context, tx SQLTx) (unix int64, err error)
	Renew(ctx context.Context, tx SQLTx, cluster string, workerID int64, leaseSeconds int) error
	Release(ctx context.Context, tx SQLTx, cluster string, workerID int64) error
}
