package workerid

import (
	"context"
	"errors"
	"fmt"
)

// SQLGenerator is a SQL-backed worker ID allocator.
// The concrete ORM/driver is selected by SQLClient; the database product by SQLDialect.
type SQLGenerator struct {
	client       SQLClient
	dialect      SQLDialect
	cluster      string
	maxWorkerID  uint32
	leaseSeconds int
}

var _ Generator = (*SQLGenerator)(nil)

// NewSQLGenerator creates a SQLGenerator.
// It does not create tables or seed the ID pool; apply schema migrations and
// call InitializeSQLCluster before use.
func NewSQLGenerator(client SQLClient, dialect SQLDialect, cluster string, options ...Option) (*SQLGenerator, error) {
	if client == nil {
		return nil, errors.New("sql client is nil")
	}
	if dialect == nil {
		return nil, errors.New("sql dialect is nil")
	}
	cfg, err := ResolveConfig(cluster, options...)
	if err != nil {
		return nil, err
	}
	return &SQLGenerator{
		client:       client,
		dialect:      dialect,
		cluster:      cfg.Cluster,
		maxWorkerID:  cfg.MaxWorkerID,
		leaseSeconds: int(cfg.MaxLeaseTime.Seconds()),
	}, nil
}

func (g *SQLGenerator) begin(ctx context.Context) (SQLTx, error) {
	tx, err := g.client.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	if err := g.dialect.PrepareTx(ctx, tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// GetID acquires an available worker ID.
func (g *SQLGenerator) GetID() (int64, string, error) {
	ctx := context.Background()
	tx, err := g.begin(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	workerID, err := g.dialect.SelectAvailable(ctx, tx, g.cluster)
	if errors.Is(err, ErrSQLNoRows) {
		return 0, "", ErrNoAvailableID
	}
	if err != nil {
		return 0, "", fmt.Errorf("select available lease: %w", err)
	}

	token := generateToken()
	if err := g.dialect.Claim(ctx, tx, g.cluster, workerID, token, g.leaseSeconds); err != nil {
		return 0, "", fmt.Errorf("claim lease: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("commit: %w", err)
	}
	return workerID, token, nil
}

func (g *SQLGenerator) validateLease(ctx context.Context, tx SQLTx, workerID int64, token string) error {
	storedToken, expireAt, err := g.dialect.SelectLease(ctx, tx, g.cluster, workerID)
	if errors.Is(err, ErrSQLNoRows) {
		return ErrNotAssigned
	}
	if err != nil {
		return fmt.Errorf("select lease: %w", err)
	}
	if storedToken == "" {
		return ErrNotAssigned
	}
	if storedToken != token {
		return ErrTokenMismatch
	}

	now, err := g.dialect.Now(ctx, tx)
	if err != nil {
		return fmt.Errorf("select current timestamp: %w", err)
	}
	if expireAt <= now {
		return ErrTokenExpired
	}
	return nil
}

// Renew extends the lease of a worker ID.
func (g *SQLGenerator) Renew(workerID int64, token string) error {
	if workerID < 0 || workerID > int64(g.maxWorkerID) {
		return ErrInvalidWorkerID
	}
	if len(token) != 22 {
		return ErrInvalidToken
	}

	ctx := context.Background()
	tx, err := g.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := g.validateLease(ctx, tx, workerID, token); err != nil {
		return err
	}
	if err := g.dialect.Renew(ctx, tx, g.cluster, workerID, g.leaseSeconds); err != nil {
		return fmt.Errorf("renew lease: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Release releases a worker ID back to the pool.
func (g *SQLGenerator) Release(workerID int64, token string) error {
	if workerID < 0 || workerID > int64(g.maxWorkerID) {
		return ErrInvalidWorkerID
	}
	if len(token) != 22 {
		return ErrInvalidToken
	}

	ctx := context.Background()
	tx, err := g.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := g.validateLease(ctx, tx, workerID, token); err != nil {
		return err
	}
	if err := g.dialect.Release(ctx, tx, g.cluster, workerID); err != nil {
		return fmt.Errorf("release lease: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// InitializeSQLCluster writes/validates cluster metadata and seeds the worker ID pool.
// Tables must already exist (apply the dialect schema via your migration tool).
func InitializeSQLCluster(ctx context.Context, client SQLClient, dialect SQLDialect, cluster string, options ...Option) error {
	if client == nil {
		return errors.New("sql client is nil")
	}
	if dialect == nil {
		return errors.New("sql dialect is nil")
	}
	cfg, err := ResolveConfig(cluster, options...)
	if err != nil {
		return err
	}

	tx, err := client.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := dialect.PrepareTx(ctx, tx); err != nil {
		return fmt.Errorf("prepare tx: %w", err)
	}

	existing, ok, err := dialect.LookupClusterMax(ctx, tx, cfg.Cluster)
	if err != nil {
		return fmt.Errorf("lookup cluster: %w", err)
	}
	if ok && existing != cfg.MaxWorkerID {
		return fmt.Errorf("%w: stored=%d configured=%d", ErrClusterConfigMismatch, existing, cfg.MaxWorkerID)
	}
	if !ok {
		if err := dialect.InsertCluster(ctx, tx, cfg.Cluster, cfg.MaxWorkerID); err != nil {
			return fmt.Errorf("insert cluster: %w", err)
		}
	}
	if err := dialect.SeedLeases(ctx, tx, cfg.Cluster, cfg.MaxWorkerID); err != nil {
		return fmt.Errorf("seed leases: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
