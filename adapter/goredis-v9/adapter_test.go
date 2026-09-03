package goredisv9

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"libx.net/workerid"
)

func TestNewGenerator(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	gen, err := NewGenerator(client, "test-cluster", workerid.WithWorkerBits(4), workerid.WithMaxLeaseTime(time.Minute))
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}

	workerID, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID: %v", err)
	}
	if workerID < 0 || workerID > 15 {
		t.Fatalf("unexpected workerID: %d", workerID)
	}
	if len(token) != 22 {
		t.Fatalf("unexpected token length: %d", len(token))
	}

	if err := gen.Renew(workerID, token); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if err := gen.Release(workerID, token); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestNewDoer(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	doer := NewDoer(client)
	gen, err := workerid.NewRedisGenerator(doer, "test-cluster")
	if err != nil {
		t.Fatalf("NewRedisGenerator: %v", err)
	}
	if _, _, err := gen.GetID(); err != nil {
		t.Fatalf("GetID: %v", err)
	}
}
