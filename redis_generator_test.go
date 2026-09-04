package workerid

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRedis is a minimal scripted RedisDoer for unit tests (no third-party deps).
type fakeRedis struct {
	mu        sync.Mutex
	evalSeq   []any
	evalIdx   int
	timeUnix  int64
	failBegin bool
}

func (f *fakeRedis) Do(_ context.Context, args ...any) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	cmd, _ := args[0].(string)
	switch strings.ToUpper(cmd) {
	case "TIME":
		sec := f.timeUnix
		if sec == 0 {
			sec = time.Now().Unix()
		}
		return []any{strconv.FormatInt(sec, 10), "0"}, nil
	case "EVAL":
		if f.evalIdx >= len(f.evalSeq) {
			return nil, fmt.Errorf("unexpected EVAL call %d", f.evalIdx)
		}
		reply := f.evalSeq[f.evalIdx]
		f.evalIdx++
		if err, ok := reply.(error); ok {
			return nil, err
		}
		return reply, nil
	default:
		return nil, fmt.Errorf("unsupported command %q", cmd)
	}
}

func TestNewRedisGenerator_Validation(t *testing.T) {
	if _, err := NewRedisGenerator(nil, "c"); err == nil {
		t.Fatal("expected nil client error")
	}
	if _, err := NewRedisGenerator(&fakeRedis{evalSeq: []any{int64(0)}}, ""); err == nil {
		t.Fatal("expected empty cluster error")
	}
}

func TestNewRedisGenerator_MaxLeaseTime(t *testing.T) {
	fr := &fakeRedis{evalSeq: []any{int64(1)}}
	_, err := NewRedisGenerator(fr, "c1", WithMaxLeaseTime(500*time.Millisecond))
	if !errors.Is(err, ErrMaxLeaseTimeTooShort) {
		t.Fatalf("500ms: err=%v, want ErrMaxLeaseTimeTooShort", err)
	}
	if fr.evalIdx != 0 {
		t.Fatalf("rejected lease must not initialize Redis, evalIdx=%d", fr.evalIdx)
	}

	fr = &fakeRedis{evalSeq: []any{int64(1)}}
	_, err = NewRedisGenerator(fr, "c1", WithMaxLeaseTime(999*time.Millisecond))
	if !errors.Is(err, ErrMaxLeaseTimeTooShort) {
		t.Fatalf("999ms: err=%v, want ErrMaxLeaseTimeTooShort", err)
	}

	fr = &fakeRedis{evalSeq: []any{int64(1)}}
	gen, err := NewRedisGenerator(fr, "c1", WithMaxLeaseTime(time.Second))
	if err != nil {
		t.Fatalf("1s: %v", err)
	}
	if gen.leaseSeconds != 1 {
		t.Fatalf("leaseSeconds=%d, want 1", gen.leaseSeconds)
	}

	fr = &fakeRedis{evalSeq: []any{int64(1)}}
	gen, err = NewRedisGenerator(fr, "c1")
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if gen.leaseSeconds != 300 {
		t.Fatalf("leaseSeconds=%d, want 300", gen.leaseSeconds)
	}
}

func TestRedisGenerator_GetID(t *testing.T) {
	fr := &fakeRedis{evalSeq: []any{
		int64(1), // initAvailableIDs
		int64(3), // GetID
	}}
	gen, err := NewRedisGenerator(fr, "c1", WithWorkerBits(4))
	if err != nil {
		t.Fatalf("NewRedisGenerator: %v", err)
	}
	id, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID: %v", err)
	}
	if id != 3 || len(token) != 22 {
		t.Fatalf("id=%d token=%q", id, token)
	}
}

func TestRedisGenerator_GetID_NoAvailable(t *testing.T) {
	fr := &fakeRedis{evalSeq: []any{
		int64(0),  // init
		int64(-1), // GetID none
	}}
	gen, err := NewRedisGenerator(fr, "c1")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = gen.GetID()
	if err != ErrNoAvailableID {
		t.Fatalf("err=%v, want ErrNoAvailableID", err)
	}
}

func TestRedisGenerator_RenewAndRelease(t *testing.T) {
	token := "abcdefghijklmnopqrstuv"
	fr := &fakeRedis{evalSeq: []any{
		int64(0), // init
		"OK",     // renew
		"OK",     // release
	}}
	gen, err := NewRedisGenerator(fr, "c1", WithWorkerBits(3))
	if err != nil {
		t.Fatal(err)
	}
	if err := gen.Renew(1, token); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if err := gen.Release(1, token); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestRedisGenerator_RenewErrors(t *testing.T) {
	token := "abcdefghijklmnopqrstuv"
	cases := []struct {
		status string
		want   error
	}{
		{"NOT_FOUND", ErrNotAssigned},
		{"MISMATCH", ErrTokenMismatch},
		{"EXPIRED", ErrTokenExpired},
		{"INVALID", ErrInvalidToken},
	}
	for _, tc := range cases {
		fr := &fakeRedis{evalSeq: []any{int64(0), tc.status}}
		gen, err := NewRedisGenerator(fr, "c1")
		if err != nil {
			t.Fatal(err)
		}
		if err := gen.Renew(0, token); err != tc.want {
			t.Fatalf("status %s: got %v want %v", tc.status, err, tc.want)
		}
	}
	fr := &fakeRedis{evalSeq: []any{int64(0)}}
	gen, err := NewRedisGenerator(fr, "c1", WithWorkerBits(2))
	if err != nil {
		t.Fatal(err)
	}
	if err := gen.Renew(-1, token); err != ErrInvalidWorkerID {
		t.Fatalf("invalid id: %v", err)
	}
	if err := gen.Renew(1, "short"); err != ErrInvalidToken {
		t.Fatalf("invalid token: %v", err)
	}
}

func TestRedisGenerator_WithOptions(t *testing.T) {
	fr := &fakeRedis{evalSeq: []any{int64(1)}}
	gen, err := NewRedisGenerator(fr, "c1", WithWorkerBits(10), WithMaxLeaseTime(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if gen.maxWorkerID != 1023 {
		t.Fatalf("maxWorkerID=%d", gen.maxWorkerID)
	}
	if gen.leaseSeconds != 600 {
		t.Fatalf("leaseSeconds=%d", gen.leaseSeconds)
	}
}
