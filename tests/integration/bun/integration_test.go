package bun_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"libx.net/workerid"
	bunadapter "libx.net/workerid/adapter/bun"
)

// TestIntegrationBunPostgreSQL reproduces GitHub issue #1: bunadapter.InitializeCluster
// against Postgres with PostgresDialect() must bind $n arguments.
func TestIntegrationBunPostgreSQL(t *testing.T) {
	dsn := os.Getenv("WORKERID_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WORKERID_POSTGRES_DSN not set")
	}

	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sqldb.Close()
	if err := sqldb.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	if _, err := sqldb.Exec(workerid.PostgresSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	db := bun.NewDB(sqldb, pgdialect.New())
	cluster := "it-bun-pg-" + time.Now().Format("150405.000")
	opts := []workerid.Option{workerid.WithWorkerBits(3), workerid.WithMaxLeaseTime(time.Minute)}
	ctx := context.Background()

	if err := bunadapter.InitializeCluster(ctx, db, workerid.PostgresDialect(), cluster, opts...); err != nil {
		t.Fatalf("InitializeCluster: %v", err)
	}

	gen, err := bunadapter.NewGenerator(db, workerid.PostgresDialect(), cluster, opts...)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}

	id, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID: %v", err)
	}
	if err := gen.Renew(id, token); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if err := gen.Release(id, token); err != nil {
		t.Fatalf("Release: %v", err)
	}
}
