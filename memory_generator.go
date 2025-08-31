package workerid

import (
	"sync"
	"time"
)

type workerToken struct {
	token     string
	leaseTime int64
}
type MemoryGenerator struct {
	maxWorkerID  uint32
	leaseSeconds int64
	mu           sync.Mutex
	availableIDs map[int64]int64
	tokens       map[int64]workerToken
}

func NewMemoryGenerator(options ...Option) *MemoryGenerator {
	opts := &generatorOptions{
		maxWorkerID:  1000,
		maxLeaseTime: 5 * time.Minute,
	}
	for _, o := range options {
		o(opts)
	}
	if opts.maxLeaseTime <= 0 {
		opts.maxLeaseTime = 5 * time.Minute
	}
	if opts.maxWorkerID <= 0 {
		opts.maxWorkerID = 1000
	}
	availableIDs := make(map[int64]int64, int(opts.maxWorkerID))
	for i := 1; i <= int(opts.maxWorkerID); i++ {
		availableIDs[int64(i)] = 0
	}

	return &MemoryGenerator{
		maxWorkerID:  opts.maxWorkerID,
		leaseSeconds: int64(opts.maxLeaseTime.Seconds()),
		availableIDs: availableIDs,
		mu:           sync.Mutex{},
	}
}

func (g *MemoryGenerator) GetID() (int64, string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	var workerID int64
	now := time.Now().Unix()
	newExpiresAt := now + g.leaseSeconds
	for id, expireAt := range g.availableIDs {
		if expireAt < now {
			workerID = id
			g.availableIDs[workerID] = newExpiresAt
			break
		}
	}

	token := generateToken()

	g.tokens[workerID] = workerToken{
		token:     token,
		leaseTime: newExpiresAt,
	}

	return workerID, token, nil
}

func (g *MemoryGenerator) Renew(workerID int64, token string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.tokens[workerID]; !ok {
		return ErrNotAssigned
	}
	if g.tokens[workerID].token != token {
		return ErrTokenMismatch
	}
	now := time.Now().Unix()
	if g.tokens[workerID].leaseTime < now {
		return ErrTokenExpired
	}
	newExpireAt := now + g.leaseSeconds
	g.tokens[workerID] = workerToken{
		token:     token,
		leaseTime: newExpireAt,
	}
	g.availableIDs[workerID] = newExpireAt
	return nil
}

func (g *MemoryGenerator) Release(workerID int64, token string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.tokens[workerID]; !ok {
		return ErrNotAssigned
	}
	if g.tokens[workerID].token != token {
		return ErrTokenMismatch
	}
	now := time.Now().Unix()
	if g.tokens[workerID].leaseTime < now {
		return ErrTokenExpired
	}
	delete(g.tokens, workerID)
	g.availableIDs[workerID] = 0
	return nil
}
