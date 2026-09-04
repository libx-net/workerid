package workerid

import (
	"testing"
)

func TestGenerator_Interface(t *testing.T) {
	tests := []struct {
		name      string
		generator func() Generator
	}{
		{
			name: "MemoryGenerator",
			generator: func() Generator {
				return NewMemoryGenerator()
			},
		},
		{
			name: "RedisGenerator",
			generator: func() Generator {
				fr := &fakeRedis{evalSeq: []any{
					int64(1), // init
					int64(0), // GetID
					"OK",     // Renew
					"OK",     // Release
					"NOT_FOUND", // duplicate Release
				}}
				gen, err := NewRedisGenerator(fr, "test-cluster")
				if err != nil {
					t.Fatalf("NewRedisGenerator: %v", err)
				}
				return gen
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := tt.generator()

			workerID, token, err := gen.GetID()
			if err != nil {
				t.Errorf("GetID() error: %v", err)
				return
			}
			if workerID < 0 {
				t.Errorf("WorkerID should be >= 0, got %d", workerID)
			}
			if len(token) != 22 {
				t.Errorf("Token length should be 22, got %d", len(token))
			}

			if err := gen.Renew(workerID, token); err != nil {
				t.Errorf("Renew() error: %v", err)
			}
			if err := gen.Renew(workerID+999, token); err == nil {
				t.Error("Renew() with invalid WorkerID should return error")
			}
			if err := gen.Renew(workerID, "invalid_token"); err == nil {
				t.Error("Renew() with invalid Token should return error")
			}

			if err := gen.Release(workerID, token); err != nil {
				t.Errorf("Release() error: %v", err)
			}
			if err := gen.Release(workerID+999, token); err == nil {
				t.Error("Release() with invalid WorkerID should return error")
			}
			if tt.name != "MemoryGenerator" {
				if err := gen.Release(workerID, token); err == nil {
					t.Error("Release() duplicate release should return error")
				}
			}
		})
	}
}

func TestGenerator_ErrorTypes(t *testing.T) {
	errs := []error{
		ErrNoAvailableID,
		ErrInvalidWorkerID,
		ErrTokenMismatch,
		ErrTokenExpired,
		ErrNotAssigned,
		ErrInvalidToken,
		ErrMaxLeaseTimeTooShort,
	}
	for _, err := range errs {
		if err == nil {
			t.Error("predefined error should not be nil")
		}
		if err.Error() == "" {
			t.Errorf("error %T should have non-empty message", err)
		}
	}
}

func TestGenerateToken(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := generateToken()
		if len(token) != 22 {
			t.Errorf("Token length should be 22, got %d", len(token))
		}
		if tokens[token] {
			t.Errorf("Token %s generated twice", token)
		}
		tokens[token] = true
		for _, char := range token {
			if !((char >= 'A' && char <= 'Z') ||
				(char >= 'a' && char <= 'z') ||
				(char >= '0' && char <= '9') ||
				char == '-' || char == '_') {
				t.Errorf("Token contains invalid character: %c", char)
			}
		}
	}
}
