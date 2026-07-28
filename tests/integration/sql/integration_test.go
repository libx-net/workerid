package sql_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"libx.net/workerid"
)

func TestIntegrationPostgreSQL(t *testing.T) {
	dsn := os.Getenv("WORKERID_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WORKERID_POSTGRES_DSN not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	if _, err := db.Exec(workerid.PostgresSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	client := workerid.NewDatabaseSQLClient(db)
	cluster := "it-pg-" + time.Now().Format("150405.000")
	opts := []workerid.Option{workerid.WithWorkerBits(3), workerid.WithMaxLeaseTime(time.Minute)}
	if err := workerid.InitializeSQLCluster(context.Background(), client, workerid.PostgresDialect(), cluster, opts...); err != nil {
		t.Fatalf("init: %v", err)
	}
	gen, err := workerid.NewSQLGenerator(client, workerid.PostgresDialect(), cluster, opts...)
	if err != nil {
		t.Fatalf("new: %v", err)
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

	const n = 8
	var wg sync.WaitGroup
	results := make(chan int64, n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wid, _, err := gen.GetID()
			if err != nil {
				errs <- err
				return
			}
			results <- wid
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent: %v", err)
	}
	seen := map[int64]bool{}
	for id := range results {
		if seen[id] {
			t.Fatalf("duplicate %d", id)
		}
		seen[id] = true
	}
}

func TestIntegrationMySQL(t *testing.T) {
	dsn := os.Getenv("WORKERID_MYSQL_DSN")
	if dsn == "" {
		t.Skip("WORKERID_MYSQL_DSN not set")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	if _, err := db.Exec(workerid.MySQLSchema); err != nil {
		// MySQL may not support CREATE INDEX IF NOT EXISTS on older servers;
		// fall back to splitting statements from a minimal inline schema.
		for _, stmt := range []string{
			`CREATE TABLE IF NOT EXISTS workerid_clusters (
				cluster VARCHAR(255) PRIMARY KEY,
				max_worker_id INT NOT NULL,
				created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
			)`,
			`CREATE TABLE IF NOT EXISTS workerid_leases (
				cluster VARCHAR(255) NOT NULL,
				worker_id INT NOT NULL,
				token VARCHAR(64) NULL,
				expire_at TIMESTAMP(6) NOT NULL DEFAULT '1970-01-01 00:00:00.000000',
				PRIMARY KEY (cluster, worker_id)
			)`,
		} {
			if _, err2 := db.Exec(stmt); err2 != nil {
				t.Fatalf("schema: %v / %v", err, err2)
			}
		}
	}

	client := workerid.NewDatabaseSQLClient(db)
	cluster := "it-mysql-" + time.Now().Format("150405")
	opts := []workerid.Option{workerid.WithWorkerBits(3), workerid.WithMaxLeaseTime(time.Minute)}

	dialect := workerid.MySQL80Dialect()
	if err := workerid.InitializeSQLCluster(context.Background(), client, dialect, cluster, opts...); err != nil {
		dialect = workerid.MySQL57Dialect()
		if err2 := workerid.InitializeSQLCluster(context.Background(), client, dialect, cluster, opts...); err2 != nil {
			t.Fatalf("init: %v / %v", err, err2)
		}
	}

	gen, err := workerid.NewSQLGenerator(client, dialect, cluster, opts...)
	if err != nil {
		t.Fatal(err)
	}
	id, token, err := gen.GetID()
	if err != nil {
		t.Fatal(err)
	}
	if err := gen.Renew(id, token); err != nil {
		t.Fatal(err)
	}
	if err := gen.Release(id, token); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationSQLite(t *testing.T) {
	path := os.Getenv("WORKERID_SQLITE_PATH")
	if path == "" {
		t.Skip("WORKERID_SQLITE_PATH not set")
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(workerid.SQLiteSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	client := workerid.NewDatabaseSQLClient(db)
	cluster := "it-sqlite"
	opts := []workerid.Option{workerid.WithWorkerBits(3), workerid.WithMaxLeaseTime(time.Minute)}
	if err := workerid.InitializeSQLCluster(context.Background(), client, workerid.SQLiteDialect(), cluster, opts...); err != nil {
		t.Fatalf("init: %v", err)
	}
	gen, err := workerid.NewSQLGenerator(client, workerid.SQLiteDialect(), cluster, opts...)
	if err != nil {
		t.Fatal(err)
	}
	id, token, err := gen.GetID()
	if err != nil {
		t.Fatal(err)
	}
	if err := gen.Renew(id, token); err != nil {
		t.Fatal(err)
	}
	if err := gen.Release(id, token); err != nil {
		t.Fatal(err)
	}
}
