package workerid

import (
	"context"
	"strings"
	"testing"
)

// captureSQLTx records the query passed to QueryRow.
type captureSQLTx struct {
	query string
}

func (t *captureSQLTx) QueryRow(_ context.Context, query string, _ ...any) SQLRow {
	t.query = query
	return &fakeSQLRow{vals: scanVals{int64(0)}}
}

func (t *captureSQLTx) Exec(context.Context, string, ...any) error { return nil }
func (t *captureSQLTx) Commit() error                              { return nil }
func (t *captureSQLTx) Rollback() error                            { return nil }

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
