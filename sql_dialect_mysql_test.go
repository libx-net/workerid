package workerid

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

// captureSQLTx records QueryRow and Exec SQL for dialect tests.
type captureSQLTx struct {
	query   string   // last QueryRow
	queries []string // Exec queries, in order
}

func (t *captureSQLTx) QueryRow(_ context.Context, query string, _ ...any) SQLRow {
	t.query = query
	return &fakeSQLRow{vals: scanVals{int64(0)}}
}

func (t *captureSQLTx) Exec(_ context.Context, query string, _ ...any) error {
	t.queries = append(t.queries, query)
	return nil
}

func (t *captureSQLTx) Commit() error   { return nil }
func (t *captureSQLTx) Rollback() error { return nil }

func TestMySQLSelectAvailable_LimitBeforeForUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		dialect    SQLDialect
		wantLock   string
		skipLocked bool
	}{
		{
			name:       "mysql57",
			dialect:    MySQL57Dialect(),
			wantLock:   "FOR UPDATE",
			skipLocked: false,
		},
		{
			name:       "mysql80",
			dialect:    MySQL80Dialect(),
			wantLock:   "FOR UPDATE SKIP LOCKED",
			skipLocked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tx := &captureSQLTx{}
			if _, err := tt.dialect.SelectAvailable(context.Background(), tx, "c1"); err != nil {
				t.Fatal(err)
			}

			normalized := strings.Join(strings.Fields(tx.query), " ")
			limitPos := strings.Index(normalized, "LIMIT 1")
			lockPos := strings.Index(normalized, "FOR UPDATE")
			if limitPos < 0 || lockPos < 0 {
				t.Fatalf("expected LIMIT 1 and FOR UPDATE in %q", normalized)
			}
			// Regression: old SQL was "... FOR UPDATE [SKIP LOCKED] LIMIT 1",
			// which MySQL 5.7/8.x reject with error 1064 near 'LIMIT 1'.
			if limitPos > lockPos {
				t.Fatalf("MySQL requires LIMIT before FOR UPDATE, got %q", normalized)
			}
			if !strings.Contains(normalized, tt.wantLock) {
				t.Fatalf("want lock clause %q in %q", tt.wantLock, normalized)
			}
			hasSkipLocked := strings.Contains(normalized, "SKIP LOCKED")
			if hasSkipLocked != tt.skipLocked {
				t.Fatalf("SKIP LOCKED present=%v, want %v in %q", hasSkipLocked, tt.skipLocked, normalized)
			}
		})
	}
}

func TestMySQLSchemaExpireAtDefaultIsLegal(t *testing.T) {
	// The original shipped schema used:
	//   expire_at TIMESTAMP(6) NOT NULL DEFAULT '1970-01-01 00:00:00.000000'
	// MySQL TIMESTAMP valid range starts at 1970-01-01 00:00:01 UTC, so that
	// default is rejected as Error 1067 under STRICT_TRANS_TABLES,NO_ZERO_DATE.
	illegalTSZero := regexp.MustCompile(`(?is)expire_at\s+TIMESTAMP(?:\s*\(\s*6\s*\))?\s+NOT\s+NULL\s+DEFAULT\s+'1970-01-01 00:00:00`)
	const originalBuggyColumn = "expire_at TIMESTAMP(6) NOT NULL DEFAULT '1970-01-01 00:00:00.000000'"
	if !illegalTSZero.MatchString(originalBuggyColumn) {
		t.Fatal("test regex no longer detects the original TIMESTAMP zero default")
	}
	if illegalTSZero.MatchString(MySQLSchema) {
		t.Fatal("MySQLSchema uses illegal TIMESTAMP zero default for expire_at")
	}

	datetimeExpire := regexp.MustCompile(`(?is)expire_at\s+DATETIME\s*\(\s*6\s*\)\s+NOT\s+NULL\s+DEFAULT\s+'1970-01-01 00:00:00`)
	if !datetimeExpire.MatchString(MySQLSchema) {
		t.Fatal("MySQLSchema expire_at must be DATETIME(6) with a Unix-epoch default")
	}

	if !strings.Contains(MySQLSchema, mysqlUnleasedExpireAt) {
		t.Fatalf("MySQLSchema default must use sentinel %q", mysqlUnleasedExpireAt)
	}
}

func TestMySQLSchemaDocumentsTimestampUpgrade(t *testing.T) {
	// CREATE TABLE IF NOT EXISTS is a no-op on an existing TIMESTAMP column.
	if !strings.Contains(MySQLSchema, "ALTER TABLE workerid_leases") {
		t.Fatal("MySQLSchema must document ALTER for existing TIMESTAMP expire_at")
	}
	if !strings.Contains(MySQLSchema, "MODIFY expire_at DATETIME(6) NOT NULL DEFAULT '1970-01-01 00:00:00.000000'") {
		t.Fatal("MySQLSchema must document MODIFY expire_at to DATETIME(6)")
	}
	if !strings.Contains(MySQLSchema, "ADD INDEX workerid_leases_available_idx (cluster, expire_at, worker_id)") {
		t.Fatal("MySQLSchema must document adding workerid_leases_available_idx")
	}
}

func TestMySQLSchemaDDLIsIdempotent(t *testing.T) {
	// DDL is not transactional: if workerid_clusters is created and
	// workerid_leases then fails, retry must not hit "table already exists".
	for _, table := range []string{"workerid_clusters", "workerid_leases"} {
		want := "CREATE TABLE IF NOT EXISTS " + table
		if !strings.Contains(MySQLSchema, want) {
			t.Fatalf("MySQLSchema missing %q", want)
		}
	}
	// CREATE INDEX IF NOT EXISTS is not portable to MySQL 5.7; the available
	// index must be declared on CREATE TABLE so a retry is a no-op.
	for _, line := range strings.Split(MySQLSchema, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "CREATE INDEX") {
			t.Fatal("MySQLSchema must not use standalone CREATE INDEX (not idempotent on MySQL 5.7)")
		}
	}
	if !strings.Contains(MySQLSchema, "workerid_leases_available_idx") {
		t.Fatal("MySQLSchema missing workerid_leases_available_idx")
	}
}

func TestMySQLDialectSeedAndReleaseUseDatetimeSentinel(t *testing.T) {
	d := MySQL80Dialect()
	tx := &captureSQLTx{}
	if err := d.SeedLeases(context.Background(), tx, "c", 2); err != nil {
		t.Fatal(err)
	}
	if err := d.Release(context.Background(), tx, "c", 1); err != nil {
		t.Fatal(err)
	}
	if len(tx.queries) < 2 {
		t.Fatalf("expected seed and release queries, got %d", len(tx.queries))
	}
	for _, q := range tx.queries {
		if !strings.Contains(q, mysqlUnleasedExpireAt) {
			t.Fatalf("query missing unleased sentinel %q:\n%s", mysqlUnleasedExpireAt, q)
		}
	}
}
