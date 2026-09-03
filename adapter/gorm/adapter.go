package gormadapter

import (
	"context"
	"database/sql"
	"errors"

	"gorm.io/gorm"
	"libx.net/workerid"
)

type client struct {
	db *gorm.DB
}

type tx struct {
	db *gorm.DB
}

type row struct {
	row *sql.Row
}

// NewClient wraps *gorm.DB as workerid.SQLClient.
func NewClient(db *gorm.DB) workerid.SQLClient {
	if db == nil {
		return nil
	}
	return &client{db: db}
}

// NewGenerator creates a SQLGenerator using GORM and the given dialect.
func NewGenerator(db *gorm.DB, dialect workerid.SQLDialect, cluster string, options ...workerid.Option) (*workerid.SQLGenerator, error) {
	return workerid.NewSQLGenerator(NewClient(db), dialect, cluster, options...)
}

// InitializeCluster seeds cluster metadata and the worker ID pool via GORM.
func InitializeCluster(ctx context.Context, db *gorm.DB, dialect workerid.SQLDialect, cluster string, options ...workerid.Option) error {
	return workerid.InitializeSQLCluster(ctx, NewClient(db), dialect, cluster, options...)
}

func (c *client) BeginTx(ctx context.Context) (workerid.SQLTx, error) {
	gdb := c.db.WithContext(ctx).Begin()
	if gdb.Error != nil {
		return nil, gdb.Error
	}
	return &tx{db: gdb}, nil
}

func (t *tx) QueryRow(ctx context.Context, query string, args ...any) workerid.SQLRow {
	return &row{row: t.db.WithContext(ctx).Raw(query, args...).Row()}
}

func (t *tx) Exec(ctx context.Context, query string, args ...any) error {
	return t.db.WithContext(ctx).Exec(query, args...).Error
}

func (t *tx) Commit() error {
	return t.db.Commit().Error
}

func (t *tx) Rollback() error {
	return t.db.Rollback().Error
}

func (r *row) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return workerid.ErrSQLNoRows
	}
	return err
}
