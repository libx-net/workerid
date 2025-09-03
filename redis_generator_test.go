package workerid

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	// 启动 miniredis 服务器
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("启动 miniredis 失败: %v", err)
	}

	// 创建 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		mr.Close()
		t.Fatalf("连接 Redis 失败: %v", err)
	}

	// 返回客户端和清理函数
	return client, func() {
		client.Close()
		mr.Close()
	}
}

func TestNewRedisGenerator(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	tests := []struct {
		name    string
		cluster string
		options []Option
		wantErr bool
	}{
		{
			name:    "正常创建",
			cluster: "test-cluster",
			options: nil,
			wantErr: false,
		},
		{
			name:    "自定义配置",
			cluster: "test-cluster-2",
			options: []Option{WithWorkerBits(7), WithMaxLeaseTime(10 * time.Minute)},
			wantErr: false,
		},
		{
			name:    "空集群名称",
			cluster: "",
			options: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := NewRedisGenerator(client, tt.cluster, tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRedisGenerator() 错误 = %v, 期望错误 %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gen == nil {
				t.Error("NewRedisGenerator() 返回 nil")
			}
		})
	}
}

func TestRedisGenerator_GetID(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	gen, err := NewRedisGenerator(client, "test-cluster")
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 测试获取 ID
	workerID, token, err := gen.GetID()
	if err != nil {
		t.Errorf("GetID() 返回错误: %v", err)
	}

	if workerID < 0 || workerID > int64(gen.maxWorkerID) {
		t.Errorf("WorkerID 应该在 0-%d 范围内, 实际值: %d", gen.maxWorkerID, workerID)
	}

	if len(token) != 22 {
		t.Errorf("Token 长度应该为 22, 实际长度: %d", len(token))
	}

	// 测试获取多个 ID
	workerID2, token2, err := gen.GetID()
	if err != nil {
		t.Errorf("第二次 GetID() 返回错误: %v", err)
	}

	// 应该获取到不同的 ID 或相同的 ID（取决于可用性）
	if workerID2 < 0 || workerID2 > int64(gen.maxWorkerID) {
		t.Errorf("第二个 WorkerID 应该在 0-%d 范围内, 实际值: %d", gen.maxWorkerID, workerID2)
	}

	// Token 应该不同
	if token == token2 {
		t.Error("不同的获取请求应该生成不同的 Token")
	}
}

func TestRedisGenerator_Renew(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	gen, err := NewRedisGenerator(client, "test-cluster")
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 先获取一个 ID
	workerID, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID() 失败: %v", err)
	}

	tests := []struct {
		name     string
		workerID int64
		token    string
		wantErr  bool
	}{
		{
			name:     "正确续期",
			workerID: workerID,
			token:    token,
			wantErr:  false,
		},
		{
			name:     "无效的WorkerID",
			workerID: -1,
			token:    token,
			wantErr:  true,
		},
		{
			name:     "WorkerID超出范围",
			workerID: int64(gen.maxWorkerID) + 1,
			token:    token,
			wantErr:  true,
		},
		{
			name:     "无效的Token格式",
			workerID: workerID,
			token:    "invalid",
			wantErr:  true,
		},
		{
			name:     "错误的Token",
			workerID: workerID,
			token:    "abcdefghijklmnopqrstuv",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.Renew(tt.workerID, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("Renew() 错误 = %v, 期望错误 %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedisGenerator_RenewWithRedisStateCheck(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	gen, err := NewRedisGenerator(client, "test-cluster")
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 先获取一个 ID
	workerID, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID() 失败: %v", err)
	}

	ctx := context.Background()
	idsKey := gen.getIDsKey()
	tokenKey := gen.getTokenKey()

	// 获取续期前的过期时间
	beforeScore, err := client.ZScore(ctx, idsKey, fmt.Sprintf("%d", workerID)).Result()
	if err != nil {
		t.Fatalf("获取续期前的过期时间失败: %v", err)
	}

	// 获取续期前的 Token 数据
	beforeTokenData, err := client.HGet(ctx, tokenKey, fmt.Sprintf("%d", workerID)).Result()
	if err != nil {
		t.Fatalf("获取续期前的 Token 数据失败: %v", err)
	}

	// 等待至少 1 秒，确保时间戳有差异（getCurrentTime 返回秒级时间戳）
	time.Sleep(1100 * time.Millisecond)

	// 执行续期
	err = gen.Renew(workerID, token)
	if err != nil {
		t.Fatalf("Renew() 失败: %v", err)
	}

	// 获取续期后的过期时间
	afterScore, err := client.ZScore(ctx, idsKey, fmt.Sprintf("%d", workerID)).Result()
	if err != nil {
		t.Fatalf("获取续期后的过期时间失败: %v", err)
	}

	// 获取续期后的 Token 数据
	afterTokenData, err := client.HGet(ctx, tokenKey, fmt.Sprintf("%d", workerID)).Result()
	if err != nil {
		t.Fatalf("获取续期后的 Token 数据失败: %v", err)
	}

	// 验证过期时间已更新（应该比之前的时间更大）
	if afterScore <= beforeScore {
		t.Errorf("续期后的过期时间应该更大，续期前: %f, 续期后: %f", beforeScore, afterScore)
	}

	// 验证 Token 数据已更新
	if afterTokenData == beforeTokenData {
		t.Error("续期后的 Token 数据应该已更新")
	}

	// 验证 Token 数据格式正确
	tokenParts := strings.Split(afterTokenData, ":")
	if len(tokenParts) != 2 {
		t.Errorf("Token 数据格式错误: %s", afterTokenData)
	}

	// 验证 Token 部分没有变化
	if tokenParts[0] != token {
		t.Errorf("Token 部分不应该变化，期望: %s, 实际: %s", token, tokenParts[0])
	}

	// 验证过期时间部分已更新
	newExpireTime, err := strconv.ParseFloat(tokenParts[1], 64)
	if err != nil {
		t.Fatalf("解析新的过期时间失败: %v", err)
	}

	if newExpireTime != afterScore {
		t.Errorf("Token 中的过期时间与 ZSet 中的分数不匹配，Token: %f, ZSet: %f", newExpireTime, afterScore)
	}
}

func TestRedisGenerator_Release(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	gen, err := NewRedisGenerator(client, "test-cluster")
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 先获取一个 ID
	workerID, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID() 失败: %v", err)
	}

	tests := []struct {
		name     string
		workerID int64
		token    string
		wantErr  bool
	}{
		{
			name:     "无效的WorkerID",
			workerID: -1,
			token:    token,
			wantErr:  true,
		},
		{
			name:     "WorkerID超出范围",
			workerID: int64(gen.maxWorkerID) + 1,
			token:    token,
			wantErr:  true,
		},
		{
			name:     "无效的Token格式",
			workerID: workerID,
			token:    "invalid",
			wantErr:  true,
		},
		{
			name:     "正确释放",
			workerID: workerID,
			token:    token,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.Release(tt.workerID, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("Release() 错误 = %v, 期望错误 %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedisGenerator_ReleaseWithRedisStateCheck(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	gen, err := NewRedisGenerator(client, "test-cluster")
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 先获取一个 ID
	workerID, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID() 失败: %v", err)
	}

	ctx := context.Background()
	idsKey := gen.getIDsKey()
	tokenKey := gen.getTokenKey()

	// 验证释放前的状态
	// 1. 检查 WorkerID 在 ZSet 中的分数（过期时间）应该大于 0
	beforeScore, err := client.ZScore(ctx, idsKey, fmt.Sprintf("%d", workerID)).Result()
	if err != nil {
		t.Fatalf("获取释放前的过期时间失败: %v", err)
	}
	if beforeScore <= 0 {
		t.Errorf("释放前的过期时间应该大于 0，实际值: %f", beforeScore)
	}

	// 2. 检查 Token 映射关系存在
	beforeTokenData, err := client.HGet(ctx, tokenKey, fmt.Sprintf("%d", workerID)).Result()
	if err != nil {
		t.Fatalf("获取释放前的 Token 数据失败: %v", err)
	}
	if beforeTokenData == "" {
		t.Error("释放前应该存在 Token 映射关系")
	}

	// 验证 Token 数据格式
	tokenParts := strings.Split(beforeTokenData, ":")
	if len(tokenParts) != 2 {
		t.Errorf("Token 数据格式错误: %s", beforeTokenData)
	}
	if tokenParts[0] != token {
		t.Errorf("Token 不匹配，期望: %s, 实际: %s", token, tokenParts[0])
	}

	// 执行释放
	err = gen.Release(workerID, token)
	if err != nil {
		t.Fatalf("Release() 失败: %v", err)
	}

	// 验证释放后的状态
	// 1. 检查 WorkerID 在 ZSet 中的分数应该重置为 0
	afterScore, err := client.ZScore(ctx, idsKey, fmt.Sprintf("%d", workerID)).Result()
	if err != nil {
		t.Fatalf("获取释放后的过期时间失败: %v", err)
	}
	if afterScore != 0 {
		t.Errorf("释放后的过期时间应该重置为 0，实际值: %f", afterScore)
	}

	// 2. 检查 Token 映射关系已移除
	afterTokenData, err := client.HGet(ctx, tokenKey, fmt.Sprintf("%d", workerID)).Result()
	if err != redis.Nil && err != nil {
		t.Fatalf("检查释放后的 Token 数据失败: %v", err)
	}
	if afterTokenData != "" {
		t.Errorf("释放后 Token 映射关系应该已移除，但仍存在: %s", afterTokenData)
	}

	// 3. 验证 WorkerID 可以被重新分配
	newWorkerID, newToken, err := gen.GetID()
	if err != nil {
		t.Fatalf("释放后重新获取 ID 失败: %v", err)
	}

	// 由于我们只释放了一个 ID，在小的 maxWorkerID 情况下，很可能会重新分配到同一个 ID
	if newWorkerID < 0 || newWorkerID > int64(gen.maxWorkerID) {
		t.Errorf("重新分配的 WorkerID 应该在有效范围内，实际值: %d", newWorkerID)
	}

	if newToken == token {
		t.Error("重新分配的 Token 应该与之前的不同")
	}

	// 验证新分配的 ID 在 Redis 中的状态正确
	newScore, err := client.ZScore(ctx, idsKey, fmt.Sprintf("%d", newWorkerID)).Result()
	if err != nil {
		t.Fatalf("获取重新分配的 ID 过期时间失败: %v", err)
	}
	if newScore <= 0 {
		t.Errorf("重新分配的 ID 过期时间应该大于 0，实际值: %f", newScore)
	}

	newTokenData, err := client.HGet(ctx, tokenKey, fmt.Sprintf("%d", newWorkerID)).Result()
	if err != nil {
		t.Fatalf("获取重新分配的 Token 数据失败: %v", err)
	}
	if newTokenData == "" {
		t.Error("重新分配的 ID 应该有对应的 Token 映射关系")
	}

	newTokenParts := strings.Split(newTokenData, ":")
	if len(newTokenParts) != 2 {
		t.Errorf("重新分配的 Token 数据格式错误: %s", newTokenData)
	}
	if newTokenParts[0] != newToken {
		t.Errorf("重新分配的 Token 不匹配，期望: %s, 实际: %s", newToken, newTokenParts[0])
	}
}

func TestRedisGenerator_ReleaseAndReuse(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()
	workerBits := uint(2)
	maxID := int64(3)

	gen, err := NewRedisGenerator(client, "test-cluster", WithWorkerBits(workerBits))
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 获取第一个 ID
	workerID1, token1, err := gen.GetID()
	if err != nil {
		t.Fatalf("获取第一个 ID 失败: %v", err)
	}

	// 获取第二个 ID
	_, token2, err := gen.GetID()
	if err != nil {
		t.Fatalf("获取第二个 ID 失败: %v", err)
	}

	// 释放第一个 ID
	err = gen.Release(workerID1, token1)
	if err != nil {
		t.Fatalf("释放第一个 ID 失败: %v", err)
	}

	// 应该能够重新获取到 ID（可能是之前释放的 ID）
	workerID3, token3, err := gen.GetID()
	if err != nil {
		t.Fatalf("重新获取 ID 失败: %v", err)
	}

	// 验证获取到的 ID 有效
	if workerID3 < 0 || workerID3 > maxID {
		t.Errorf("重新获取的 WorkerID 应该在 0-%d 范围内, 实际值: %d", maxID, workerID3)
	}

	// Token 应该不同
	if token3 == token1 || token3 == token2 {
		t.Error("重新获取的 Token 应该与之前的不同")
	}
}

func TestRedisGenerator_ConcurrentAccess(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	gen, err := NewRedisGenerator(client, "test-cluster", WithWorkerBits(10))
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 并发获取 ID
	const numGoroutines = 5
	results := make(chan struct {
		workerID int64
		token    string
		err      error
	}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			workerID, token, err := gen.GetID()
			results <- struct {
				workerID int64
				token    string
				err      error
			}{workerID, token, err}
		}()
	}

	// 收集结果
	workerIDs := make(map[int64]bool)
	tokens := make(map[string]bool)

	for i := 0; i < numGoroutines; i++ {
		result := <-results
		if result.err != nil {
			t.Errorf("并发获取 ID 失败: %v", result.err)
			continue
		}

		// 检查 WorkerID 唯一性（在当前时刻）
		if workerIDs[result.workerID] {
			t.Errorf("WorkerID %d 被重复分配", result.workerID)
		}
		workerIDs[result.workerID] = true

		// 检查 Token 唯一性
		if tokens[result.token] {
			t.Errorf("Token %s 被重复生成", result.token)
		}
		tokens[result.token] = true
	}
}

func TestRedisGenerator_WithOptions(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	workerBits := uint(10)
	maxWorkerID := uint32(1023)
	maxLeaseTime := 10 * time.Minute

	gen, err := NewRedisGenerator(client, "test-cluster",
		WithWorkerBits(workerBits),
		WithMaxLeaseTime(maxLeaseTime))
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 验证配置生效
	if gen.maxWorkerID != maxWorkerID {
		t.Errorf("maxWorkerID = %d, 期望 %d", gen.maxWorkerID, maxWorkerID)
	}

	if gen.leaseSeconds != int(maxLeaseTime.Seconds()) {
		t.Errorf("leaseSeconds = %d, 期望 %d", gen.leaseSeconds, int(maxLeaseTime.Seconds()))
	}

	// 测试获取 ID 在指定范围内
	workerID, _, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID() 失败: %v", err)
	}

	if workerID < 0 || workerID > int64(maxWorkerID) {
		t.Errorf("WorkerID 应该在 1-%d 范围内, 实际值: %d", maxWorkerID, workerID)
	}
}

// TestRedisGenerator_RenewHashExpiration 测试续期时 Hash 过期时间是否正确更新
func TestRedisGenerator_RenewHashExpiration(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	gen, err := NewRedisGenerator(client, "test-cluster", WithMaxLeaseTime(5*time.Second))
	if err != nil {
		t.Fatalf("创建 RedisGenerator 失败: %v", err)
	}

	// 获取 WorkerID
	workerID, token, err := gen.GetID()
	if err != nil {
		t.Fatalf("GetID() 失败: %v", err)
	}

	tokenKey := gen.getTokenKey()

	// 检查初始 Hash 过期时间
	initialTTL, err := client.TTL(context.Background(), tokenKey).Result()
	if err != nil {
		t.Fatalf("获取初始 TTL 失败: %v", err)
	}
	if initialTTL <= 0 {
		t.Fatalf("初始 TTL 应该大于 0，实际值: %v", initialTTL)
	}

	// 期望的完整 TTL 值（lease * 3 = 15 秒）
	expectedFullTTL := time.Duration(gen.leaseSeconds*3) * time.Second

	// t.Logf("初始 TTL: %v, 期望完整 TTL: %v", initialTTL, expectedFullTTL)

	// 手动设置一个较短的 TTL 来模拟接近过期的场景
	shortTTL := 3 * time.Second
	err = client.Expire(context.Background(), tokenKey, shortTTL).Err()
	if err != nil {
		t.Fatalf("设置短 TTL 失败: %v", err)
	}

	// 验证 TTL 已经被设置为较短的值
	beforeRenewTTL, err := client.TTL(context.Background(), tokenKey).Result()
	if err != nil {
		t.Fatalf("获取续期前 TTL 失败: %v", err)
	}

	if beforeRenewTTL > shortTTL+time.Second {
		t.Fatalf("TTL 应该被设置为较短的值，期望约 %v，实际: %v", shortTTL, beforeRenewTTL)
	}

	// t.Logf("手动设置短 TTL 后: %v", beforeRenewTTL)

	// 执行续期
	err = gen.Renew(workerID, token)
	if err != nil {
		t.Fatalf("Renew() 失败: %v", err)
	}

	// 检查续期后 Hash 过期时间是否重新设置
	afterRenewTTL, err := client.TTL(context.Background(), tokenKey).Result()
	if err != nil {
		t.Fatalf("获取续期后 TTL 失败: %v", err)
	}

	// 关键验证1：续期后的 TTL 应该比续期前的 TTL 大很多（证明续期生效）
	if afterRenewTTL <= beforeRenewTTL {
		t.Errorf("续期后 TTL 应该比续期前大，续期前: %v, 续期后: %v", beforeRenewTTL, afterRenewTTL)
	}

	// 关键验证2：续期后 TTL 应该接近完整的期望值
	if afterRenewTTL < expectedFullTTL-2*time.Second {
		t.Errorf("续期后 TTL 应该接近完整值 %v，实际值: %v", expectedFullTTL, afterRenewTTL)
	}

	// 关键验证3：续期的效果应该显著（TTL 增加应该很明显）
	ttlIncrease := afterRenewTTL - beforeRenewTTL
	minExpectedIncrease := 8 * time.Second // 从 3 秒增加到 15 秒，至少应该增加 8 秒
	if ttlIncrease < minExpectedIncrease {
		t.Errorf("续期效果不够显著，TTL 增加量: %v, 期望至少增加: %v", ttlIncrease, minExpectedIncrease)
	}

	// 验证 Hash 内容仍然存在且正确
	tokenData, err := client.HGet(context.Background(), tokenKey, strconv.Itoa(int(workerID))).Result()
	if err != nil {
		t.Fatalf("获取续期后 token 数据失败: %v", err)
	}
	if tokenData == "" {
		t.Fatalf("续期后 token 数据不应该为空")
	}

	// t.Logf("续期验证成功 - 续期前: %v, 续期后: %v, TTL 增加: %v, Hash 数据存在: %v",
	// 	beforeRenewTTL, afterRenewTTL, ttlIncrease, tokenData != "")
}

func TestWithWorkerBits(t *testing.T) {
	workerBits := map[uint]uint32{
		4:  15,
		6:  63,
		8:  255,
		10: 1023,
		12: 4095,
	}
	s := miniredis.RunT(t)
	defer s.Close()
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	for b, m := range workerBits {
		// 为每个测试用例创建独立的集群名称，避免数据冲突
		clusterName := fmt.Sprintf("test-cluster-%d", b)

		g, err := NewRedisGenerator(client, clusterName, WithWorkerBits(b))
		if err != nil {
			t.Errorf("创建 RedisGenerator 失败: %v", err)
			continue
		}

		if g.maxWorkerID == 0 {
			t.Errorf("maxWorkerID 应该大于 0, 实际值: %d", g.maxWorkerID)
		}
		if g.maxWorkerID != m {
			t.Errorf("maxWorkerID 应该为 %d, 实际值: %d", m, g.maxWorkerID)
		}

		// 检查 Redis 中的 ID 范围
		ctx := context.Background()
		idsKey := fmt.Sprintf("{workerid:cluster:%s}:ids", clusterName)

		// 验证 ZSet 中的成员总数
		totalCount, err := client.ZCard(ctx, idsKey).Result()
		if err != nil {
			t.Errorf("获取 Redis ZSet 成员总数失败: %v", err)
			continue
		}

		expectedCount := int64(m) + 1 // 从 0 到 maxWorkerID，总共 maxWorkerID + 1 个
		if totalCount != expectedCount {
			t.Errorf("WorkerBits=%d: Redis 中的成员数量应该为 %d, 实际值: %d",
				b, expectedCount, totalCount)
			continue
		}

		// 验证最小的 WorkerID (0) 存在
		exists, err := client.ZScore(ctx, idsKey, "0").Result()
		if err != nil {
			t.Errorf("WorkerBits=%d: 最小 WorkerID '0' 应该存在于 Redis 中: %v", b, err)
			continue
		}
		if exists != 0 {
			t.Errorf("WorkerBits=%d: WorkerID '0' 的初始分数应该是 0, 实际值: %f", b, exists)
		}

		// 验证最大的 WorkerID (maxWorkerID) 存在
		maxWorkerIDStr := strconv.Itoa(int(m))
		exists, err = client.ZScore(ctx, idsKey, maxWorkerIDStr).Result()
		if err != nil {
			t.Errorf("WorkerBits=%d: 最大 WorkerID '%s' 应该存在于 Redis 中: %v", b, maxWorkerIDStr, err)
			continue
		}
		if exists != 0 {
			t.Errorf("WorkerBits=%d: WorkerID '%s' 的初始分数应该是 0, 实际值: %f", b, maxWorkerIDStr, exists)
		}

		// 验证超出范围的 WorkerID 不存在
		outOfRangeID := strconv.Itoa(int(m) + 1)
		_, err = client.ZScore(ctx, idsKey, outOfRangeID).Result()
		if !errors.Is(err, redis.Nil) {
			t.Errorf("WorkerBits=%d: 超出范围的 WorkerID '%s' 不应该存在于 Redis 中", b, outOfRangeID)
		}

		// 抽样验证中间的一些 WorkerID 存在且分数为 0
		sampleIDs := []int{1, int(m) / 4, int(m) / 2, int(m) * 3 / 4}
		for _, sampleID := range sampleIDs {
			if sampleID <= int(m) {
				sampleIDStr := strconv.Itoa(sampleID)
				score, err := client.ZScore(ctx, idsKey, sampleIDStr).Result()
				if err != nil {
					t.Errorf("WorkerBits=%d: WorkerID '%s' 应该存在于 Redis 中: %v", b, sampleIDStr, err)
					continue
				}
				if score != 0 {
					t.Errorf("WorkerBits=%d: WorkerID '%s' 的初始分数应该是 0, 实际值: %f",
						b, sampleIDStr, score)
				}
			}
		}
	}
}
