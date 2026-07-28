package workerid

import (
	"testing"
)

func TestNewMemoryGenerator(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		wantErr bool
	}{
		{
			name:    "default configuration",
			options: nil,
			wantErr: false,
		},
		{
			name:    "custom max WorkerID",
			options: []Option{WithWorkerBits(4)},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMemoryGenerator(tt.options...)
			if gen == nil {
				t.Error("NewMemoryGenerator() returned nil")
				return
			}

			// Verify generated WorkerID is within valid range
			if gen.workerID <= 0 {
				t.Errorf("WorkerID should be > 0, got: %d", gen.workerID)
			}

			// Verify Token is non-empty and has correct length
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

	// Verify generated WorkerID is within specified range
	if gen.workerID <= 0 || gen.workerID > int64(maxWorkerID) {
		t.Errorf("WorkerID should be in range 1-%d, got: %d", maxWorkerID, gen.workerID)
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
