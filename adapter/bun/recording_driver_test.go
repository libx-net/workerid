package bunadapter

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/dialect/feature"
	"github.com/uptrace/bun/schema"
)

var (
	_ driver.Driver         = (*recordingDriver)(nil)
	_ driver.Conn           = (*recordingConn)(nil)
	_ driver.ConnBeginTx    = (*recordingConn)(nil)
	_ driver.QueryerContext = (*recordingConn)(nil)
	_ driver.ExecerContext  = (*recordingConn)(nil)
)

type recordedCall struct {
	query string
	nargs int
	args  []driver.Value
}

type argRecorder struct {
	mu    sync.Mutex
	items []recordedCall
}

func (r *argRecorder) observe(query string, args []driver.NamedValue) error {
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	r.mu.Lock()
	r.items = append(r.items, recordedCall{query: query, nargs: len(args), args: vals})
	r.mu.Unlock()

	want := dollarPlaceholderCount(query)
	if want != len(args) {
		return fmt.Errorf("expected %d arguments, got %d", want, len(args))
	}
	return nil
}

func (r *argRecorder) last() recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) == 0 {
		return recordedCall{}
	}
	return r.items[len(r.items)-1]
}

func (r *argRecorder) calls() []recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedCall, len(r.items))
	copy(out, r.items)
	return out
}

var dollarNum = regexp.MustCompile(`\$(\d+)`)

func dollarPlaceholderCount(query string) int {
	max := 0
	for _, m := range dollarNum.FindAllStringSubmatch(query, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max
}

var driverSeq atomic.Int64

type recordingDriver struct {
	rec *argRecorder
}

func (d *recordingDriver) Open(string) (driver.Conn, error) {
	return &recordingConn{rec: d.rec}, nil
}

type recordingConn struct {
	rec *argRecorder
}

func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected Prepare; QueryerContext/ExecerContext should be used")
}

func (c *recordingConn) Close() error { return nil }

func (c *recordingConn) Begin() (driver.Tx, error) { return nopTx{}, nil }

func (c *recordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return nopTx{}, nil
}

func (c *recordingConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.rec.observe(query, args); err != nil {
		return nil, err
	}
	return &emptyRows{}, nil
}

func (c *recordingConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.rec.observe(query, args); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

type nopTx struct{}

func (nopTx) Commit() error   { return nil }
func (nopTx) Rollback() error { return nil }

type emptyRows struct{}

func (emptyRows) Columns() []string { return []string{"col"} }
func (emptyRows) Close() error      { return nil }
func (emptyRows) Next([]driver.Value) error {
	return io.EOF
}

func openRecordingBun(t *testing.T) (*bun.DB, *argRecorder) {
	t.Helper()
	rec := &argRecorder{}
	name := fmt.Sprintf("workerid-bun-record-%d", driverSeq.Add(1))
	sql.Register(name, &recordingDriver{rec: rec})
	sqldb, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })
	return bun.NewDB(sqldb, newStubPGDialect()), rec
}

// stubPGDialect is a non-nop bun dialect so format() would interpolate "?" if
// present. Queries that only use $n are left unchanged — the same path as
// bun pgdialect with workerid.PostgresDialect.
type stubPGDialect struct {
	schema.BaseDialect
	tables *schema.Tables
}

func newStubPGDialect() *stubPGDialect {
	d := new(stubPGDialect)
	d.tables = schema.NewTables(d)
	return d
}

func (d *stubPGDialect) Init(*sql.DB) {}

func (d *stubPGDialect) Name() dialect.Name { return dialect.PG }

func (d *stubPGDialect) Features() feature.Feature { return 0 }

func (d *stubPGDialect) Tables() *schema.Tables { return d.tables }

func (d *stubPGDialect) OnTable(*schema.Table) {}

func (d *stubPGDialect) IdentQuote() byte { return '"' }

func (d *stubPGDialect) AppendSequence(b []byte, _ *schema.Table, _ *schema.Field) []byte {
	return b
}

func (d *stubPGDialect) DefaultVarcharLen() int { return 0 }

func (d *stubPGDialect) DefaultSchema() string { return "public" }
