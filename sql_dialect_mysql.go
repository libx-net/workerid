package workerid

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"
)

//go:embed schema_mysql.sql
var MySQLSchema string

type mysqlDialect struct {
	skipLocked bool
}

// MySQL57Dialect returns a SQLDialect for MySQL 5.7 (blocking FOR UPDATE).
func MySQL57Dialect() SQLDialect { return &mysqlDialect{skipLocked: false} }

// MySQL80Dialect returns a SQLDialect for MySQL 8.0+ (FOR UPDATE SKIP LOCKED).
func MySQL80Dialect() SQLDialect { return &mysqlDialect{skipLocked: true} }

func (d *mysqlDialect) PrepareTx(context.Context, SQLTx) error { return nil }

func (d *mysqlDialect) LookupClusterMax(ctx context.Context, tx SQLTx, cluster string) (uint32, bool, error) {
	var max uint32
	err := tx.QueryRow(ctx, `
SELECT max_worker_id FROM workerid_clusters WHERE cluster = ?
`, cluster).Scan(&max)
	if errors.Is(err, ErrSQLNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return max, true, nil
}

func (d *mysqlDialect) InsertCluster(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error {
	return tx.Exec(ctx, `
INSERT IGNORE INTO workerid_clusters (cluster, max_worker_id)
VALUES (?, ?)
`, cluster, maxWorkerID)
}

func (d *mysqlDialect) SeedLeases(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error {
	const batchSize = 256
	for start := uint32(0); start <= maxWorkerID; start += batchSize {
		end := start + batchSize - 1
		if end > maxWorkerID {
			end = maxWorkerID
		}
		query := "INSERT IGNORE INTO workerid_leases (cluster, worker_id, token, expire_at) VALUES "
		args := make([]any, 0, int(end-start+1)*2)
		for id := start; id <= end; id++ {
			if id > start {
				query += ","
			}
			query += "(?, ?, NULL, '1970-01-01 00:00:00.000000')"
			args = append(args, cluster, id)
		}
		if err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("seed batch [%d,%d]: %w", start, end, err)
		}
		if end == maxWorkerID {
			break
		}
	}
	return nil
}

func (d *mysqlDialect) SelectAvailable(ctx context.Context, tx SQLTx, cluster string) (int64, error) {
	lockClause := "FOR UPDATE"
	if d.skipLocked {
		lockClause = "FOR UPDATE SKIP LOCKED"
	}
	// MySQL requires LIMIT before FOR UPDATE [SKIP LOCKED].
	// Putting LIMIT after FOR UPDATE is a syntax error (1064) on 5.7 and 8.x.
	query := fmt.Sprintf(`
SELECT worker_id
FROM workerid_leases
WHERE cluster = ? AND expire_at <= CURRENT_TIMESTAMP(6)
ORDER BY worker_id
LIMIT 1
%s
`, lockClause)
	var workerID int64
	err := tx.QueryRow(ctx, query, cluster).Scan(&workerID)
	return workerID, err
}

func (d *mysqlDialect) Claim(ctx context.Context, tx SQLTx, cluster string, workerID int64, token string, leaseSeconds int) error {
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET token = ?, expire_at = CURRENT_TIMESTAMP(6) + INTERVAL ? SECOND
WHERE cluster = ? AND worker_id = ?
`, token, leaseSeconds, cluster, workerID)
}

func (d *mysqlDialect) SelectLease(ctx context.Context, tx SQLTx, cluster string, workerID int64) (string, int64, error) {
	var token sql.NullString
	var expireAt time.Time
	err := tx.QueryRow(ctx, `
SELECT token, expire_at
FROM workerid_leases
WHERE cluster = ? AND worker_id = ?
FOR UPDATE
`, cluster, workerID).Scan(&token, &expireAt)
	if err != nil {
		return "", 0, err
	}
	return token.String, expireAt.Unix(), nil
}

func (d *mysqlDialect) Now(ctx context.Context, tx SQLTx) (int64, error) {
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT CURRENT_TIMESTAMP(6)`).Scan(&now); err != nil {
		return 0, err
	}
	return now.Unix(), nil
}

func (d *mysqlDialect) Renew(ctx context.Context, tx SQLTx, cluster string, workerID int64, leaseSeconds int) error {
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET expire_at = CURRENT_TIMESTAMP(6) + INTERVAL ? SECOND
WHERE cluster = ? AND worker_id = ?
`, leaseSeconds, cluster, workerID)
}

func (d *mysqlDialect) Release(ctx context.Context, tx SQLTx, cluster string, workerID int64) error {
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET token = NULL, expire_at = '1970-01-01 00:00:00.000000'
WHERE cluster = ? AND worker_id = ?
`, cluster, workerID)
}
