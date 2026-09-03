package workerid

import (
	"errors"
	"time"
)

type generatorOptions struct {
	cluster      string
	maxWorkerID  uint32
	maxLeaseTime time.Duration
}

// GeneratorConfig holds resolved generator settings after applying options.
type GeneratorConfig struct {
	Cluster      string
	MaxWorkerID  uint32
	MaxLeaseTime time.Duration
}

type Option func(*generatorOptions)

func WithWorkerBits(workerBits uint) Option {
	return func(o *generatorOptions) {
		o.maxWorkerID = 1<<workerBits - 1
	}
}

func WithMaxLeaseTime(maxLeaseTime time.Duration) Option {
	return func(o *generatorOptions) {
		o.maxLeaseTime = maxLeaseTime
	}
}

// ResolveConfig applies options with the same defaults used by all generators.
func ResolveConfig(cluster string, options ...Option) (GeneratorConfig, error) {
	opts := &generatorOptions{
		cluster:      cluster,
		maxWorkerID:  511, // default 512 worker IDs, max WorkerID is 511
		maxLeaseTime: 5 * time.Minute,
	}
	for _, o := range options {
		o(opts)
	}
	if opts.cluster == "" {
		return GeneratorConfig{}, errors.New("cluster is empty")
	}
	if opts.maxLeaseTime <= 0 {
		opts.maxLeaseTime = 5 * time.Minute
	}
	if opts.maxWorkerID <= 0 {
		opts.maxWorkerID = 511
	}
	return GeneratorConfig{
		Cluster:      opts.cluster,
		MaxWorkerID:  opts.maxWorkerID,
		MaxLeaseTime: opts.maxLeaseTime,
	}, nil
}
