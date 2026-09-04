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

// ErrMaxLeaseTimeTooShort is returned when WithMaxLeaseTime is set to a
// positive duration shorter than 1s. Redis and SQL backends store the lease as
// whole seconds (int(duration.Seconds())), so sub-second values would become 0
// and the ID would be immediately expired/reclaimable.
var ErrMaxLeaseTimeTooShort = errors.New("max lease time must be at least 1s")

// ResolveConfig applies options with the same defaults used by all generators.
// Zero or negative MaxLeaseTime still defaults to 5 minutes. Positive values
// shorter than 1s are rejected rather than truncated to 0 lease seconds.
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
	} else if opts.maxLeaseTime < time.Second {
		return GeneratorConfig{}, ErrMaxLeaseTimeTooShort
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
