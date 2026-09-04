package workerid

import (
	"math"
	"testing"
)

func TestNewMemoryGenerator(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		maxID   int64
	}{
		{
			name:    "default configuration",
			options: nil,
			maxID:   511,
		},
		{
			name:    "custom max WorkerID",
			options: []Option{WithWorkerBits(4)},
			maxID:   15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMemoryGenerator(tt.options...)
			if gen == nil {
				t.Error("NewMemoryGenerator() returned nil")
				return
			}

			if gen.workerID < 0 || gen.workerID > tt.maxID {
				t.Errorf("WorkerID should be in [0, %d], got: %d", tt.maxID, gen.workerID)
			}

			if len(gen.token) != 22 {
				t.Errorf("Token length should be 22, got: %d", len(gen.token))
			}
		})
	}
}

func TestMemoryGenerator_GetID(t *testing.T) {
	gen := NewMemoryGenerator()

	workerID, token, err := gen.GetID()
	if err != nil {
		t.Errorf("GetID() returned error: %v", err)
	}

	if workerID != gen.workerID {
		t.Errorf("GetID() returned WorkerID = %d, want %d", workerID, gen.workerID)
	}

	if token != gen.token {
		t.Errorf("GetID() returned Token = %s, want %s", token, gen.token)
	}
}

func TestMemoryGenerator_Renew(t *testing.T) {
	gen := NewMemoryGenerator()

	tests := []struct {
		name     string
		workerID int64
		token    string
		wantErr  error
	}{
		{
			name:     "correct WorkerID and Token",
			workerID: gen.workerID,
			token:    gen.token,
			wantErr:  nil,
		},
		{
			name:     "wrong WorkerID",
			workerID: gen.workerID + 1,
			token:    gen.token,
			wantErr:  ErrInvalidWorkerID,
		},
		{
			name:     "wrong Token",
			workerID: gen.workerID,
			token:    "invalid_token",
			wantErr:  ErrTokenMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.Renew(tt.workerID, tt.token)
			if err != tt.wantErr {
				t.Errorf("Renew() error = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryGenerator_Release(t *testing.T) {
	gen := NewMemoryGenerator()

	tests := []struct {
		name     string
		workerID int64
		token    string
		wantErr  error
	}{
		{
			name:     "correct WorkerID and Token",
			workerID: gen.workerID,
			token:    gen.token,
			wantErr:  nil,
		},
		{
			name:     "wrong WorkerID",
			workerID: gen.workerID + 1,
			token:    gen.token,
			wantErr:  ErrInvalidWorkerID,
		},
		{
			name:     "wrong Token",
			workerID: gen.workerID,
			token:    "invalid_token",
			wantErr:  ErrTokenMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.Release(tt.workerID, tt.token)
			if err != tt.wantErr {
				t.Errorf("Release() error = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryGenerator_WithMaxWorkerID(t *testing.T) {
	workerBits := uint(6)
	maxWorkerID := uint32(63)
	gen := NewMemoryGenerator(WithWorkerBits(workerBits))

	if gen.workerID < 0 || gen.workerID > int64(maxWorkerID) {
		t.Errorf("WorkerID should be in range [0, %d], got: %d", maxWorkerID, gen.workerID)
	}
}

func TestRandomWorkerID_ZeroIsReachable(t *testing.T) {
	// rand.N(1) is always 0, so maxID=0 is a deterministic construct-path
	// proof that 0 is in the inclusive range [0, maxID].
	if got := randomWorkerID(0); got != 0 {
		t.Errorf("randomWorkerID(0) = %d, want 0", got)
	}
	for i := 0; i < 8; i++ {
		if got := randomWorkerID(0); got != 0 {
			t.Errorf("randomWorkerID(0) = %d, want 0", got)
		}
	}
}

func TestRandomWorkerID_MaxUint32DoesNotPanic(t *testing.T) {
	// uint64(maxID)+1 must not wrap to 0; rand.N(0) panics.
	for i := 0; i < 4; i++ {
		_ = randomWorkerID(math.MaxUint32)
	}
}

func TestMemoryGenerator_WorkerIDZeroReachable(t *testing.T) {
	const samples = 500
	seenZero := false
	var maxSeen int64 = -1
	for i := 0; i < samples; i++ {
		gen := NewMemoryGenerator(WithWorkerBits(1)) // maxID = 1, set {0, 1}
		if gen.workerID < 0 || gen.workerID > 1 {
			t.Fatalf("WorkerID out of [0, 1]: %d", gen.workerID)
		}
		if gen.workerID == 0 {
			seenZero = true
		}
		if gen.workerID > maxSeen {
			maxSeen = gen.workerID
		}
	}
	if !seenZero {
		t.Fatalf("WorkerID 0 never appeared in %d samples with WithWorkerBits(1)", samples)
	}
	if maxSeen != 1 {
		t.Fatalf("expected WorkerID 1 to appear, maxSeen=%d", maxSeen)
	}
}

func TestMemoryGenerator_MultipleInstances(t *testing.T) {
	// Test that multiple instances generate different Tokens
	gen1 := NewMemoryGenerator()
	gen2 := NewMemoryGenerator()

	if gen1.token == gen2.token {
		t.Error("different instances should generate different Tokens")
	}

	// WorkerID may be the same (random), but Token should differ
	if gen1.token == gen2.token {
		t.Error("Tokens from different instances should not be the same")
	}
}
