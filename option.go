package workerid

import (
	"time"
)

type generatorOptions struct {
	cluster      string
	maxWorkerID  uint32
	maxLeaseTime time.Duration
}

type Option func(*generatorOptions)

func WithMaxWorkerID(maxWorkers uint32) Option {
	return func(o *generatorOptions) {
		o.maxWorkerID = maxWorkers
	}
}

func WithMaxLeaseTime(maxLeaseTime time.Duration) Option {
	return func(o *generatorOptions) {
		o.maxLeaseTime = maxLeaseTime
	}
}
