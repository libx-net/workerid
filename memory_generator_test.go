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
			name:    "默认配置",
			options: nil,
			wantErr: false,
		},
		{
			name:    "自定义最大WorkerID",
			options: []Option{WithWorkerBits(4)},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMemoryGenerator(tt.options...)
			if gen == nil {
				t.Error("NewMemoryGenerator() 返回 nil")
				return
			}

			// 验证生成的 WorkerID 在有效范围内
			if gen.workerID <= 0 {
				t.Errorf("WorkerID 应该大于 0, 实际值: %d", gen.workerID)
			}

			// 验证 Token 不为空且长度正确
			if len(gen.token) != 22 {
				t.Errorf("Token 长度应该为 22, 实际长度: %d", len(gen.token))
			}
		})
	}
}

func TestMemoryGenerator_GetID(t *testing.T) {
	gen := NewMemoryGenerator()

	workerID, token, err := gen.GetID()
	if err != nil {
		t.Errorf("GetID() 返回错误: %v", err)
	}

	if workerID != gen.workerID {
		t.Errorf("GetID() 返回的 WorkerID = %d, 期望 %d", workerID, gen.workerID)
	}

	if token != gen.token {
		t.Errorf("GetID() 返回的 Token = %s, 期望 %s", token, gen.token)
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
			name:     "正确的WorkerID和Token",
			workerID: gen.workerID,
			token:    gen.token,
			wantErr:  nil,
		},
		{
			name:     "错误的WorkerID",
			workerID: gen.workerID + 1,
			token:    gen.token,
			wantErr:  ErrInvalidWorkerID,
		},
		{
			name:     "错误的Token",
			workerID: gen.workerID,
			token:    "invalid_token",
			wantErr:  ErrTokenMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.Renew(tt.workerID, tt.token)
			if err != tt.wantErr {
				t.Errorf("Renew() 错误 = %v, 期望错误 %v", err, tt.wantErr)
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
			name:     "正确的WorkerID和Token",
			workerID: gen.workerID,
			token:    gen.token,
			wantErr:  nil,
		},
		{
			name:     "错误的WorkerID",
			workerID: gen.workerID + 1,
			token:    gen.token,
			wantErr:  ErrInvalidWorkerID,
		},
		{
			name:     "错误的Token",
			workerID: gen.workerID,
			token:    "invalid_token",
			wantErr:  ErrTokenMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.Release(tt.workerID, tt.token)
			if err != tt.wantErr {
				t.Errorf("Release() 错误 = %v, 期望错误 %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryGenerator_WithMaxWorkerID(t *testing.T) {
	workerBits := uint(6)
	maxWorkerID := uint32(63)
	gen := NewMemoryGenerator(WithWorkerBits(workerBits))

	// 验证生成的 WorkerID 在指定范围内
	if gen.workerID <= 0 || gen.workerID > int64(maxWorkerID) {
		t.Errorf("WorkerID 应该在 1-%d 范围内, 实际值: %d", maxWorkerID, gen.workerID)
	}
}

func TestMemoryGenerator_MultipleInstances(t *testing.T) {
	// 测试多个实例生成不同的 Token
	gen1 := NewMemoryGenerator()
	gen2 := NewMemoryGenerator()

	if gen1.token == gen2.token {
		t.Error("不同实例应该生成不同的 Token")
	}

	// WorkerID 可能相同（随机生成），但 Token 应该不同
	if gen1.token == gen2.token {
		t.Error("不同实例的 Token 不应该相同")
	}
}
