package workerid

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type scanVals []any

type fakeSQLRow struct {
	vals scanVals
	err  error
}

func (r *fakeSQLRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return errors.New("scan arity mismatch")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *int64:
			*d = r.vals[i].(int64)
		case *uint32:
			*d = r.vals[i].(uint32)
		case *string:
			*d = r.vals[i].(string)
		case *time.Time:
			*d = r.vals[i].(time.Time)
		default:
			return errors.New("unsupported scan dest")
		}
	}
	return nil
}

type fakeSQLTx struct {
	client *fakeSQLClient
	closed bool
}

func (t *fakeSQLTx) QueryRow(_ context.Context, _ string, _ ...any) SQLRow {
	row := t.client.popRow()
	return row
}

func (t *fakeSQLTx) Exec(_ context.Context, _ string, _ ...any) error {
	return t.client.popExec()
}

func (t *fakeSQLTx) Commit() error {
	t.closed = true
	return nil
}

func (t *fakeSQLTx) Rollback() error {
	t.closed = true
	return nil
}

type fakeSQLClient struct {
	mu     sync.Mutex
	rows   []*fakeSQLRow
	execs  []error
	begins int
}

func newFakeSQLClient() *fakeSQLClient { return &fakeSQLClient{} }

func (c *fakeSQLClient) BeginTx(context.Context) (SQLTx, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.begins++
	return &fakeSQLTx{client: c}, nil
}

func (c *fakeSQLClient) queueRow(vals scanVals, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, &fakeSQLRow{vals: vals, err: err})
}

func (c *fakeSQLClient) queueExec(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.execs = append(c.execs, err)
}

func (c *fakeSQLClient) popRow() *fakeSQLRow {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.rows) == 0 {
		return &fakeSQLRow{err: errors.New("no queued row")}
	}
	r := c.rows[0]
	c.rows = c.rows[1:]
	return r
}

func (c *fakeSQLClient) popExec() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.execs) == 0 {
		return nil
	}
	err := c.execs[0]
	c.execs = c.execs[1:]
	return err
}

// scriptDialect is a minimal dialect that ignores SQL and uses queued fake responses.
type scriptDialect struct{}

func (d *scriptDialect) PrepareTx(context.Context, SQLTx) error { return nil }

func (d *scriptDialect) LookupClusterMax(ctx context.Context, tx SQLTx, cluster string) (uint32, bool, error) {
	var max uint32
	err := tx.QueryRow(ctx, "lookup", cluster).Scan(&max)
	if errors.Is(err, ErrSQLNoRows) {
		return 0, false, nil
	}
	return max, err == nil, err
}

func (d *scriptDialect) InsertCluster(ctx context.Context, tx SQLTx, cluster string, max uint32) error {
	return tx.Exec(ctx, "insert", cluster, max)
}

func (d *scriptDialect) SeedLeases(ctx context.Context, tx SQLTx, cluster string, max uint32) error {
	return tx.Exec(ctx, "seed", cluster, max)
}

func (d *scriptDialect) SelectAvailable(ctx context.Context, tx SQLTx, cluster string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, "available", cluster).Scan(&id)
	return id, err
}

func (d *scriptDialect) Claim(ctx context.Context, tx SQLTx, cluster string, workerID int64, token string, leaseSeconds int) error {
	return tx.Exec(ctx, "claim", cluster, workerID, token, leaseSeconds)
}

func (d *scriptDialect) SelectLease(ctx context.Context, tx SQLTx, cluster string, workerID int64) (string, int64, error) {
	var token string
	var exp int64
	err := tx.QueryRow(ctx, "lease", cluster, workerID).Scan(&token, &exp)
	return token, exp, err
}

func (d *scriptDialect) Now(ctx context.Context, tx SQLTx) (int64, error) {
	var now int64
	err := tx.QueryRow(ctx, "now").Scan(&now)
	return now, err
}

func (d *scriptDialect) Renew(ctx context.Context, tx SQLTx, cluster string, workerID int64, leaseSeconds int) error {
	return tx.Exec(ctx, "renew", cluster, workerID, leaseSeconds)
}

func (d *scriptDialect) Release(ctx context.Context, tx SQLTx, cluster string, workerID int64) error {
	return tx.Exec(ctx, "release", cluster, workerID)
}

func TestSQLGenerator_GetID(t *testing.T) {
	client := newFakeSQLClient()
	client.queueRow(scanVals{int64(2)}, nil)
	client.queueExec(nil)

	gen, err := NewSQLGenerator(client, &scriptDialect{}, "c1", WithWorkerBits(3))
	if err != nil {
		t.Fatal(err)
	}
	id, token, err := gen.GetID()
	if err != nil {
		t.Fatal(err)
	}
	if id != 2 || len(token) != 22 {
		t.Fatalf("id=%d token=%q", id, token)
	}
}

func TestSQLGenerator_GetID_NoAvailable(t *testing.T) {
	client := newFakeSQLClient()
	client.queueRow(nil, ErrSQLNoRows)
	gen, err := NewSQLGenerator(client, &scriptDialect{}, "c1")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = gen.GetID()
	if !errors.Is(err, ErrNoAvailableID) {
		t.Fatalf("err=%v", err)
	}
}

func TestInitializeSQLCluster_Mismatch(t *testing.T) {
	client := newFakeSQLClient()
	client.queueRow(scanVals{uint32(15)}, nil)
	err := InitializeSQLCluster(context.Background(), client, &scriptDialect{}, "c1", WithWorkerBits(3))
	if !errors.Is(err, ErrClusterConfigMismatch) {
		t.Fatalf("err=%v", err)
	}
}

func TestInitializeSQLCluster_New(t *testing.T) {
	client := newFakeSQLClient()
	client.queueRow(nil, ErrSQLNoRows)
	client.queueExec(nil) // insert
	client.queueExec(nil) // seed
	if err := InitializeSQLCluster(context.Background(), client, &scriptDialect{}, "c1", WithWorkerBits(3)); err != nil {
		t.Fatal(err)
	}
}

func TestSQLGenerator_RenewRelease(t *testing.T) {
	token := "abcdefghijklmnopqrstuv"
	now := time.Now().Unix()
	client := newFakeSQLClient()
	// Renew
	client.queueRow(scanVals{token, now + 60}, nil)
	client.queueRow(scanVals{now}, nil)
	client.queueExec(nil)
	// Release
	client.queueRow(scanVals{token, now + 60}, nil)
	client.queueRow(scanVals{now}, nil)
	client.queueExec(nil)

	gen, err := NewSQLGenerator(client, &scriptDialect{}, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if err := gen.Renew(1, token); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if err := gen.Release(1, token); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestSQLGenerator_TokenMismatch(t *testing.T) {
	client := newFakeSQLClient()
	client.queueRow(scanVals{"xxxxxxxxxxxxxxxxxxxxxx", time.Now().Unix() + 60}, nil)
	gen, err := NewSQLGenerator(client, &scriptDialect{}, "c1")
	if err != nil {
		t.Fatal(err)
	}
	err = gen.Renew(1, "abcdefghijklmnopqrstuv")
	if !errors.Is(err, ErrTokenMismatch) {
		t.Fatalf("err=%v", err)
	}
}

func TestSQLSchemasExported(t *testing.T) {
	if PostgresSchema == "" || MySQLSchema == "" || SQLiteSchema == "" {
		t.Fatal("schemas should be embedded")
	}
	_ = PostgresDialect()
	_ = MySQL57Dialect()
	_ = MySQL80Dialect()
	_ = SQLiteDialect()
}

func TestNewDatabaseSQLClient_Nil(t *testing.T) {
	if NewDatabaseSQLClient(nil) != nil {
		t.Fatal("expected nil client for nil db")
	}
}
