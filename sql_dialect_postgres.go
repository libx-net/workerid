package workerid

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"time"
)

//go:embed schema_postgres.sql
var PostgresSchema string

type postgresDialect struct{}

// PostgresDialect returns a SQLDialect for PostgreSQL ($n placeholders, SKIP LOCKED).
func PostgresDialect() SQLDialect { return &postgresDialect{} }

func (d *postgresDialect) PrepareTx(context.Context, SQLTx) error { return nil }

func (d *postgresDialect) LookupClusterMax(ctx context.Context, tx SQLTx, cluster string) (uint32, bool, error) {
	var max uint32
	err := tx.QueryRow(ctx, `
SELECT max_worker_id FROM workerid_clusters WHERE cluster = $1
`, cluster).Scan(&max)
	if errors.Is(err, ErrSQLNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return max, true, nil
}

func (d *postgresDialect) InsertCluster(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error {
	return tx.Exec(ctx, `
INSERT INTO workerid_clusters (cluster, max_worker_id)
VALUES ($1, $2)
ON CONFLICT (cluster) DO NOTHING
`, cluster, maxWorkerID)
}

func (d *postgresDialect) SeedLeases(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error {
	return tx.Exec(ctx, `
INSERT INTO workerid_leases (cluster, worker_id, token, expire_at)
SELECT $1, g, NULL, TIMESTAMPTZ 'epoch'
FROM generate_series(0, $2) AS g
ON CONFLICT (cluster, worker_id) DO NOTHING
`, cluster, maxWorkerID)
}

func (d *postgresDialect) SelectAvailable(ctx context.Context, tx SQLTx, cluster string) (int64, error) {
	var workerID int64
	err := tx.QueryRow(ctx, `
SELECT worker_id
FROM workerid_leases
WHERE cluster = $1 AND expire_at <= CURRENT_TIMESTAMP
ORDER BY worker_id
FOR UPDATE SKIP LOCKED
LIMIT 1
`, cluster).Scan(&workerID)
	return workerID, err
}

func (d *postgresDialect) Claim(ctx context.Context, tx SQLTx, cluster string, workerID int64, token string, leaseSeconds int) error {
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET token = $3, expire_at = CURRENT_TIMESTAMP + ($4 * INTERVAL '1 second')
WHERE cluster = $1 AND worker_id = $2
`, cluster, workerID, token, leaseSeconds)
}

func (d *postgresDialect) SelectLease(ctx context.Context, tx SQLTx, cluster string, workerID int64) (string, int64, error) {
	var token sql.NullString
	var expireAt time.Time
	err := tx.QueryRow(ctx, `
SELECT token, expire_at
FROM workerid_leases
WHERE cluster = $1 AND worker_id = $2
FOR UPDATE
`, cluster, workerID).Scan(&token, &expireAt)
	if err != nil {
		return "", 0, err
	}
	return token.String, expireAt.Unix(), nil
}

func (d *postgresDialect) Now(ctx context.Context, tx SQLTx) (int64, error) {
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&now); err != nil {
		return 0, err
	}
	return now.Unix(), nil
}

func (d *postgresDialect) Renew(ctx context.Context, tx SQLTx, cluster string, workerID int64, leaseSeconds int) error {
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET expire_at = CURRENT_TIMESTAMP + ($3 * INTERVAL '1 second')
WHERE cluster = $1 AND worker_id = $2
`, cluster, workerID, leaseSeconds)
}

func (d *postgresDialect) Release(ctx context.Context, tx SQLTx, cluster string, workerID int64) error {
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET token = NULL, expire_at = TIMESTAMPTZ 'epoch'
WHERE cluster = $1 AND worker_id = $2
`, cluster, workerID)
}
