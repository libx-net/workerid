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
