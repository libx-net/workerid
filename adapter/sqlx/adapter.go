package sqlxadapter

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"libx.net/workerid"
)

type client struct {
	db *sqlx.DB
}

type tx struct {
	tx *sqlx.Tx
}

type row struct {
	row *sql.Row
}

// NewClient wraps *sqlx.DB as workerid.SQLClient.
func NewClient(db *sqlx.DB) workerid.SQLClient {
	if db == nil {
		return nil
	}
	return &client{db: db}
}

// NewGenerator creates a SQLGenerator using sqlx and the given dialect.
func NewGenerator(db *sqlx.DB, dialect workerid.SQLDialect, cluster string, options ...workerid.Option) (*workerid.SQLGenerator, error) {
	return workerid.NewSQLGenerator(NewClient(db), dialect, cluster, options...)
}

// InitializeCluster seeds cluster metadata and the worker ID pool via sqlx.
func InitializeCluster(ctx context.Context, db *sqlx.DB, dialect workerid.SQLDialect, cluster string, options ...workerid.Option) error {
	return workerid.InitializeSQLCluster(ctx, NewClient(db), dialect, cluster, options...)
}

func (c *client) BeginTx(ctx context.Context) (workerid.SQLTx, error) {
	stx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &tx{tx: stx}, nil
}

func (t *tx) QueryRow(ctx context.Context, query string, args ...any) workerid.SQLRow {
	return &row{row: t.tx.QueryRowContext(ctx, query, args...)}
}

func (t *tx) Exec(ctx context.Context, query string, args ...any) error {
	_, err := t.tx.ExecContext(ctx, query, args...)
	return err
}

func (t *tx) Commit() error   { return t.tx.Commit() }
func (t *tx) Rollback() error { return t.tx.Rollback() }

func (r *row) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return workerid.ErrSQLNoRows
	}
	return err
}
