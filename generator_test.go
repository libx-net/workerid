package workerid

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

// TestGenerator_Interface 测试所有 Generator 实现的接口一致性
func TestGenerator_Interface(t *testing.T) {
	tests := []struct {
		name      string
		generator func() Generator
		cleanup   func()
	}{
		{
			name: "MemoryGenerator",
			generator: func() Generator {
				return NewMemoryGenerator()
			},
			cleanup: func() {},
		},
		{
			name: "RedisGenerator",
			generator: func() Generator {
				mr, err := miniredis.Run()
				if err != nil {
					t.Fatalf("启动 miniredis 失败: %v", err)
				}

				client := redis.NewClient(&redis.Options{
					Addr: mr.Addr(),
				})

				gen, err := NewRedisGenerator(client, "test-cluster")
				if err != nil {
					mr.Close()
					client.Close()
					t.Fatalf("创建 RedisGenerator 失败: %v", err)
				}

				// 设置清理函数
				t.Cleanup(func() {
					client.Close()
					mr.Close()
				})

				return gen
			},
			cleanup: func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.cleanup()
			gen := tt.generator()

			// 测试 GetID
			workerID, token, err := gen.GetID()
			if err != nil {
				t.Errorf("GetID() 返回错误: %v", err)
				return
			}

			if workerID < 0 {
				t.Errorf("WorkerID 应该大于等于 0, 实际值: %d", workerID)
			}

			if len(token) != 22 {
				t.Errorf("Token 长度应该为 22, 实际长度: %d", len(token))
			}

			// 测试 Renew
			err = gen.Renew(workerID, token)
			if err != nil {
				t.Errorf("Renew() 返回错误: %v", err)
			}

			// 测试无效参数的 Renew
			err = gen.Renew(workerID+999, token)
			if err == nil {
				t.Error("Renew() 使用无效 WorkerID 应该返回错误")
			}

			err = gen.Renew(workerID, "invalid_token")
			if err == nil {
				t.Error("Renew() 使用无效 Token 应该返回错误")
			}

			// 测试 Release
			err = gen.Release(workerID, token)
			if err != nil {
				t.Errorf("Release() 返回错误: %v", err)
			}

			// 测试无效参数的 Release
			err = gen.Release(workerID+999, token)
			if err == nil {
				t.Error("Release() 使用无效 WorkerID 应该返回错误")
			}

			// 对于已释放的 ID，再次释放应该返回错误（除了 MemoryGenerator）
			if tt.name != "MemoryGenerator" {
				err = gen.Release(workerID, token)
				if err == nil {
					t.Error("Release() 重复释放应该返回错误")
				}
			}
		})
	}
}

// TestGenerator_ErrorTypes 测试错误类型的一致性
func TestGenerator_ErrorTypes(t *testing.T) {
	// 测试预定义的错误类型
	errors := []error{
		ErrNoAvailableID,
		ErrInvalidWorkerID,
		ErrTokenMismatch,
		ErrTokenExpired,
		ErrNotAssigned,
		ErrInvalidToken,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("预定义错误不应该为 nil")
		}
		if err.Error() == "" {
			t.Errorf("错误 %T 应该有非空的错误消息", err)
		}
	}
}

// TestGenerateToken 测试 Token 生成函数
func TestGenerateToken(t *testing.T) {
	// 生成多个 Token 测试唯一性
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := generateToken()

		// 检查长度
		if len(token) != 22 {
			t.Errorf("Token 长度应该为 22, 实际长度: %d", len(token))
		}

		// 检查唯一性
		if tokens[token] {
			t.Errorf("Token %s 重复生成", token)
		}
		tokens[token] = true

		// 检查字符集（base64 URL 编码）
		for _, char := range token {
			if !((char >= 'A' && char <= 'Z') ||
				(char >= 'a' && char <= 'z') ||
				(char >= '0' && char <= '9') ||
				char == '-' || char == '_') {
				t.Errorf("Token 包含无效字符: %c", char)
			}
		}
	}
}
