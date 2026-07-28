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
	randomUint32 := uint32(rand.N(uint64(maxId))) + 1

	return &MemoryGenerator{
		workerID: int64(randomUint32),
		token:    generateToken(),
	}
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
