package workerid

import (
	"math/rand/v2"
)

// MemoryGenerator is a simplified single-process implementation.
type MemoryGenerator struct {
	workerID int64
	token    string
}

func NewMemoryGenerator(options ...Option) *MemoryGenerator {
	opts := &generatorOptions{
		maxWorkerID: 511,
	}
	for _, option := range options {
		option(opts)
	}
	maxId := opts.maxWorkerID
	if maxId <= 0 {
		maxId = 511
	}

	return &MemoryGenerator{
		workerID: int64(randomWorkerID(maxId)),
		token:    generateToken(),
	}
}

// randomWorkerID returns a worker ID in the inclusive range [0, maxID],
// matching Redis/SQL. maxID is converted to uint64 before adding 1 so
// math.MaxUint32 does not overflow the argument to rand.N (which panics on n==0).
func randomWorkerID(maxID uint32) uint32 {
	return uint32(rand.N(uint64(maxID) + 1))
}

func (g *MemoryGenerator) GetID() (int64, string, error) {
	return g.workerID, g.token, nil
}

func (g *MemoryGenerator) Renew(workerID int64, token string) error {
	if workerID != g.workerID {
		return ErrInvalidWorkerID
	}
	if token != g.token {
		return ErrTokenMismatch
	}
	return nil // no-op in single-process mode
}

func (g *MemoryGenerator) Release(workerID int64, token string) error {
	if workerID != g.workerID {
		return ErrInvalidWorkerID
	}
	if token != g.token {
		return ErrTokenMismatch
	}
	return nil // no-op in single-process mode
}
