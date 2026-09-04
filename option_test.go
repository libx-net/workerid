package workerid

import (
	"errors"
	"testing"
	"time"
)

func TestResolveConfig_MaxLeaseTime(t *testing.T) {
	t.Run("500ms rejected", func(t *testing.T) {
		_, err := ResolveConfig("c", WithMaxLeaseTime(500*time.Millisecond))
		if !errors.Is(err, ErrMaxLeaseTimeTooShort) {
			t.Fatalf("err=%v, want ErrMaxLeaseTimeTooShort", err)
		}
	})
	t.Run("999ms rejected", func(t *testing.T) {
		_, err := ResolveConfig("c", WithMaxLeaseTime(999*time.Millisecond))
		if !errors.Is(err, ErrMaxLeaseTimeTooShort) {
			t.Fatalf("err=%v, want ErrMaxLeaseTimeTooShort", err)
		}
	})
	t.Run("1s accepted", func(t *testing.T) {
		cfg, err := ResolveConfig("c", WithMaxLeaseTime(time.Second))
		if err != nil {
			t.Fatalf("ResolveConfig: %v", err)
		}
		if cfg.MaxLeaseTime != time.Second {
			t.Fatalf("MaxLeaseTime=%v, want 1s", cfg.MaxLeaseTime)
		}
	})
	t.Run("default 5m", func(t *testing.T) {
		cfg, err := ResolveConfig("c")
		if err != nil {
			t.Fatalf("ResolveConfig: %v", err)
		}
		if cfg.MaxLeaseTime != 5*time.Minute {
			t.Fatalf("MaxLeaseTime=%v, want 5m", cfg.MaxLeaseTime)
		}
	})
	t.Run("zero still defaults to 5m", func(t *testing.T) {
		cfg, err := ResolveConfig("c", WithMaxLeaseTime(0))
		if err != nil {
			t.Fatalf("ResolveConfig: %v", err)
		}
		if cfg.MaxLeaseTime != 5*time.Minute {
			t.Fatalf("MaxLeaseTime=%v, want 5m", cfg.MaxLeaseTime)
		}
	})
}
