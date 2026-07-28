package workerid

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"
)

//go:embed schema_sqlite.sql
var SQLiteSchema string

type sqliteDialect struct{}

// SQLiteDialect returns a SQLDialect for SQLite (single-writer reserved lock).
func SQLiteDialect() SQLDialect { return &sqliteDialect{} }

func (d *sqliteDialect) PrepareTx(ctx context.Context, tx SQLTx) error {
	// Escalate deferred → reserved without changing rows.
	return tx.Exec(ctx, `UPDATE workerid_clusters SET cluster = cluster WHERE 0`)
}

func (d *sqliteDialect) LookupClusterMax(ctx context.Context, tx SQLTx, cluster string) (uint32, bool, error) {
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

func (d *sqliteDialect) InsertCluster(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error {
	return tx.Exec(ctx, `
INSERT OR IGNORE INTO workerid_clusters (cluster, max_worker_id)
VALUES (?, ?)
`, cluster, maxWorkerID)
}

func (d *sqliteDialect) SeedLeases(ctx context.Context, tx SQLTx, cluster string, maxWorkerID uint32) error {
	const batchSize = 256
	for start := uint32(0); start <= maxWorkerID; start += batchSize {
		end := start + batchSize - 1
		if end > maxWorkerID {
			end = maxWorkerID
		}
		query := "INSERT OR IGNORE INTO workerid_leases (cluster, worker_id, token, expire_at) VALUES "
		args := make([]any, 0, int(end-start+1)*2)
		for id := start; id <= end; id++ {
			if id > start {
				query += ","
			}
			query += "(?, ?, NULL, '1970-01-01 00:00:00')"
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

func (d *sqliteDialect) SelectAvailable(ctx context.Context, tx SQLTx, cluster string) (int64, error) {
	var workerID int64
	err := tx.QueryRow(ctx, `
SELECT worker_id
FROM workerid_leases
WHERE cluster = ? AND expire_at <= datetime('now')
ORDER BY worker_id
LIMIT 1
`, cluster).Scan(&workerID)
	return workerID, err
}

func (d *sqliteDialect) Claim(ctx context.Context, tx SQLTx, cluster string, workerID int64, token string, leaseSeconds int) error {
	modifier := fmt.Sprintf("+%d seconds", leaseSeconds)
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET token = ?, expire_at = datetime('now', ?)
WHERE cluster = ? AND worker_id = ?
`, token, modifier, cluster, workerID)
}

func (d *sqliteDialect) SelectLease(ctx context.Context, tx SQLTx, cluster string, workerID int64) (string, int64, error) {
	var token sql.NullString
	var expireAtStr string
	err := tx.QueryRow(ctx, `
SELECT token, expire_at
FROM workerid_leases
WHERE cluster = ? AND worker_id = ?
`, cluster, workerID).Scan(&token, &expireAtStr)
	if err != nil {
		return "", 0, err
	}
	expireAt, err := parseSQLiteTime(expireAtStr)
	if err != nil {
		return "", 0, err
	}
	return token.String, expireAt.Unix(), nil
}

func (d *sqliteDialect) Now(ctx context.Context, tx SQLTx) (int64, error) {
	var nowStr string
	if err := tx.QueryRow(ctx, `SELECT datetime('now')`).Scan(&nowStr); err != nil {
		return 0, err
	}
	now, err := parseSQLiteTime(nowStr)
	if err != nil {
		return 0, err
	}
	return now.Unix(), nil
}

func (d *sqliteDialect) Renew(ctx context.Context, tx SQLTx, cluster string, workerID int64, leaseSeconds int) error {
	modifier := fmt.Sprintf("+%d seconds", leaseSeconds)
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET expire_at = datetime('now', ?)
WHERE cluster = ? AND worker_id = ?
`, modifier, cluster, workerID)
}

func (d *sqliteDialect) Release(ctx context.Context, tx SQLTx, cluster string, workerID int64) error {
	return tx.Exec(ctx, `
UPDATE workerid_leases
SET token = NULL, expire_at = '1970-01-01 00:00:00'
WHERE cluster = ? AND worker_id = ?
`, cluster, workerID)
}

func parseSQLiteTime(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse sqlite time %q", s)
}
