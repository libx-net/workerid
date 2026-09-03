package bunadapter

import (
	"context"
	"testing"
	"time"

	"libx.net/workerid"
)

func TestNewClient_Nil(t *testing.T) {
	if NewClient(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestNewGenerator_NilDB(t *testing.T) {
	_, err := NewGenerator(nil, workerid.PostgresDialect(), "c")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitializeCluster_NilClient(t *testing.T) {
	err := InitializeCluster(context.Background(), nil, workerid.MySQL57Dialect(), "c")
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestQueryRowForwardsDollarArgs would fail on bun.Tx.QueryRowContext: that
// method format()s "?" then executes with zero args, so Postgres $1 never
// reaches the driver ("expected 1 arguments, got 0").
func TestQueryRowForwardsDollarArgs(t *testing.T) {
	db, rec := openRecordingBun(t)
	ctx := context.Background()

	tx, err := NewClient(db).BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	const query = `SELECT max_worker_id FROM workerid_clusters WHERE cluster = $1`
	_ = tx.QueryRow(ctx, query, "bound-cluster").Scan(new(uint32))

	call := rec.last()
	if call.nargs != 1 {
		t.Fatalf("QueryRow dropped bound args: query=%q nargs=%d want 1", call.query, call.nargs)
	}
	if call.args[0] != "bound-cluster" {
		t.Fatalf("arg[0]=%#v want %q", call.args[0], "bound-cluster")
	}
	if call.query != query {
		t.Fatalf("query rewritten to %q, want placeholders left for the driver", call.query)
	}
}

func TestExecForwardsDollarArgs(t *testing.T) {
	db, rec := openRecordingBun(t)
	ctx := context.Background()

	tx, err := NewClient(db).BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	const query = `INSERT INTO workerid_clusters (cluster, max_worker_id) VALUES ($1, $2)`
	if err := tx.Exec(ctx, query, "bound-cluster", int64(7)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	call := rec.last()
	if call.nargs != 2 {
		t.Fatalf("Exec dropped bound args: query=%q nargs=%d want 2", call.query, call.nargs)
	}
	if call.args[0] != "bound-cluster" {
		t.Fatalf("arg[0]=%#v want %q", call.args[0], "bound-cluster")
	}
	if call.args[1] != int64(7) {
		t.Fatalf("arg[1]=%#v want 7", call.args[1])
	}
}

func TestInitializeCluster_PostgresDollarPlaceholders(t *testing.T) {
	db, rec := openRecordingBun(t)
	ctx := context.Background()

	err := InitializeCluster(ctx, db, workerid.PostgresDialect(), "init-cluster",
		workerid.WithWorkerBits(3), workerid.WithMaxLeaseTime(time.Minute))
	if err != nil {
		t.Fatalf("InitializeCluster: %v", err)
	}

	var sawLookup bool
	for _, call := range rec.calls() {
		if dollarPlaceholderCount(call.query) == 0 {
			continue
		}
		if call.nargs == 0 {
			t.Fatalf("bound arguments dropped: query=%q nargs=0", call.query)
		}
		if call.nargs >= 1 && call.args[0] == "init-cluster" {
			sawLookup = true
		}
	}
	if !sawLookup {
		t.Fatal("expected a $n query bound to cluster name init-cluster")
	}
}
