package goredisv8

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
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

func TestNewGenerator_ClusterConfig(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	const cluster = "bits-cluster"
	idsKey := "{workerid:cluster:" + cluster + "}:ids"
	metaKey := "{workerid:cluster:" + cluster + "}:max_worker_id"

	if _, err := NewGenerator(client, cluster, workerid.WithWorkerBits(4), workerid.WithMaxLeaseTime(time.Minute)); err != nil {
		t.Fatalf("first NewGenerator (4 bits): %v", err)
	}
	assertSeededIDs(t, mr, idsKey, 15)
	if got, err := mr.Get(metaKey); err != nil || got != "15" {
		t.Fatalf("max_worker_id=%q err=%v, want 15", got, err)
	}

	if _, err := NewGenerator(client, cluster, workerid.WithWorkerBits(4), workerid.WithMaxLeaseTime(time.Minute)); err != nil {
		t.Fatalf("same bits should succeed: %v", err)
	}
	assertSeededIDs(t, mr, idsKey, 15)

	_, err = NewGenerator(client, cluster, workerid.WithWorkerBits(3), workerid.WithMaxLeaseTime(time.Minute))
	if !errors.Is(err, workerid.ErrClusterConfigMismatch) {
		t.Fatalf("different bits: err=%v, want ErrClusterConfigMismatch", err)
	}
	assertSeededIDs(t, mr, idsKey, 15)
	if got, err := mr.Get(metaKey); err != nil || got != "15" {
		t.Fatalf("mismatch must not rewrite max_worker_id: got=%q err=%v", got, err)
	}
}

func assertSeededIDs(t *testing.T, mr *miniredis.Miniredis, idsKey string, maxID int) {
	t.Helper()
	list, err := mr.ZMembers(idsKey)
	if err != nil {
		t.Fatalf("ZMembers %s: %v", idsKey, err)
	}
	members := make(map[string]struct{}, len(list))
	for _, m := range list {
		members[m] = struct{}{}
	}
	if len(members) != maxID+1 {
		t.Fatalf("%s has %d members, want %d (0..%d)", idsKey, len(members), maxID+1, maxID)
	}
	for i := 0; i <= maxID; i++ {
		if _, ok := members[strconv.Itoa(i)]; !ok {
			t.Fatalf("%s missing member %d", idsKey, i)
		}
	}
}
