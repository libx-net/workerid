package workerid

import (
	"context"
	"database/sql"
	"errors"
)

type databaseSQLClient struct {
	db *sql.DB
}

type databaseSQLTx struct {
	tx *sql.Tx
}

type databaseSQLRow struct {
	row *sql.Row
}

// NewDatabaseSQLClient wraps *sql.DB as SQLClient using only the standard library.
func NewDatabaseSQLClient(db *sql.DB) SQLClient {
	if db == nil {
		return nil
	}
	return &databaseSQLClient{db: db}
}

func (c *databaseSQLClient) BeginTx(ctx context.Context) (SQLTx, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &databaseSQLTx{tx: tx}, nil
}

func (t *databaseSQLTx) QueryRow(ctx context.Context, query string, args ...any) SQLRow {
	return &databaseSQLRow{row: t.tx.QueryRowContext(ctx, query, args...)}
}

func (t *databaseSQLTx) Exec(ctx context.Context, query string, args ...any) error {
	_, err := t.tx.ExecContext(ctx, query, args...)
	return err
}

func (t *databaseSQLTx) Commit() error   { return t.tx.Commit() }
func (t *databaseSQLTx) Rollback() error { return t.tx.Rollback() }

func (r *databaseSQLRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSQLNoRows
	}
	return err
}
