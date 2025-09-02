package workerid

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisGenerator struct {
	cluster      string
	maxWorkerID  uint32
	leaseSeconds int
	redisClient  *redis.Client
	ctx          context.Context
	clockSync    bool
	lockKey      string
	lockVal      string
}

var _ Generator = (*RedisGenerator)(nil)

// NewRedisGenerator 创建 RedisGenerator 实例
func NewRedisGenerator(redisClient *redis.Client, cluster string, options ...Option) (*RedisGenerator, error) {
	opts := &generatorOptions{
		cluster:      cluster,
		maxWorkerID:  1000,
		maxLeaseTime: 5 * time.Minute,
	}
	for _, o := range options {
		o(opts)
	}
	if opts.cluster == "" {
		return nil, errors.New("cluster is empty")
	}
	if opts.maxLeaseTime <= 0 {
		opts.maxLeaseTime = 5 * time.Minute
	}
	if opts.maxWorkerID <= 0 {
		opts.maxWorkerID = 1000
	}

	allocator := &RedisGenerator{
		cluster:      opts.cluster,
		maxWorkerID:  opts.maxWorkerID,
		leaseSeconds: int(opts.maxLeaseTime.Seconds()),
		redisClient:  redisClient,
		ctx:          context.Background(),
		lockKey:      fmt.Sprintf("{workerid:cluster:%s}:lock", opts.cluster),
		lockVal:      generateToken(),
	}

	if err := allocator.initAvailableIDs(); err != nil {
		return nil, fmt.Errorf("initialize available IDs failed: %w", err)
	}

	return allocator, nil
}

func (g *RedisGenerator) getCurrentTime() (int64, error) {
	if g.clockSync {
		t, err := g.redisClient.Time(g.ctx).Result()
		if err != nil {
			return 0, err
		}
		return t.Unix(), nil
	}
	return time.Now().Unix(), nil
}

func (g *RedisGenerator) initAvailableIDs() error {
	key := g.getIDsKey()
	// 如果 key 已经存在，则直接返回
	if n, err := g.redisClient.ZCard(g.ctx, key).Result(); err == nil && n > 0 {
		return nil
	}
	pipe := g.redisClient.Pipeline()
	for i := 1; i <= int(g.maxWorkerID); i++ {
		pipe.ZAdd(g.ctx, key, &redis.Z{
			Score:  0,
			Member: strconv.Itoa(i),
		})
	}
	_, err := pipe.Exec(g.ctx)
	return err
}

// getIDsKey 获取存储 WorkerID 的 Sorted Set 键
func (g *RedisGenerator) getIDsKey() string {
	return fmt.Sprintf("{workerid:cluster:%s}:ids", g.cluster)
}

// getTokenKey 获取 Token 存储键
func (g *RedisGenerator) getTokenKey() string {
	return fmt.Sprintf("{workerid:cluster:%s}:tokens", g.cluster)
}

/*
// acquireLock 获取分布式锁（SETNX + EXPIRE）
func (g *RedisGenerator) acquireLock() (bool, error) {
	// 使用 SET key value NX PX timeout 保证原子性
	result, err := g.redisClient.SetNX(g.ctx, g.lockKey, g.lockVal, time.Millisecond*5).Result()
	if err != nil {
		return false, fmt.Errorf("redis set lock failed: %w", err)
	}
	return result, nil
}

var releaseScript = redis.NewScript(`
	if redis.call('GET', KEYS[1]) == ARGV[1] then
		return redis.call('DEL', KEYS[1])
	else
		return 0
	end
`)

// releaseLock 释放分布式锁（Lua 脚本保证原子性）
func (g *RedisGenerator) releaseLock() error {
	// 使用 Lua 脚本避免误删其他客户端的锁
	_, err := releaseScript.Run(g.ctx, g.redisClient, []string{g.lockKey}, g.lockVal).Result()
	return err
}
*/

var getIDScript = redis.NewScript(`
	local key = KEYS[1]
	local now = tonumber(ARGV[1])
	local lease = tonumber(ARGV[2])

	-- 查找最小可用 ID
	local ids = redis.call('ZRANGEBYSCORE', key, '-inf', now, 'WITHSCORES', 'LIMIT', 0, 1)
	if #ids == 0 then return nil end

	local workerID = ids[1]
	local newExpire = now + lease

	-- 更新 ID 状态
	redis.call('ZADD', key, newExpire, workerID)

	-- 存储 Token
	local tokenKey = KEYS[2]
	local token = ARGV[3]
	local tokenData = token .. ':' .. newExpire
	redis.call('HSET', tokenKey, workerID, tokenData)
	redis.call('EXPIRE', tokenKey, lease * 2)  -- 设置 Token 过期时间

	return workerID
`)

func (g *RedisGenerator) GetID() (int64, string, error) {
	token := generateToken()
	now, err := g.getCurrentTime()
	if err != nil {
		return 0, "", fmt.Errorf("get current time failed: %w", err)
	}
	result, err := getIDScript.Run(g.ctx, g.redisClient, []string{g.getIDsKey(), g.getTokenKey()},
		now, g.leaseSeconds, token).Int64()
	if err != nil {
		return 0, "", fmt.Errorf("get ID failed: %w", err)
	}
	return result, token, nil
}

var renewScript = redis.NewScript(`
	local tokenKey = KEYS[1]
	local key = KEYS[2]
	local workerID = ARGV[1]
	local token = ARGV[2]
	local now = tonumber(ARGV[3])
	local lease = tonumber(ARGV[4])

	-- 1. 获取 Token 记录
	local tokenStr = redis.call('HGET', tokenKey, workerID)
	if not tokenStr then
		return {err="Token not found"}
	end

	-- 调试日志：输出原始tokenStr
	redis.log(redis.LOG_NOTICE, "DEBUG: tokenStr = " .. tostring(tokenStr))

	-- 更可靠的字符串分割方式
	local colonPos = string.find(tokenStr, ":")
	if not colonPos then
		return {err="Invalid token format"}
	end
	local storedToken = string.sub(tokenStr, 1, colonPos-1)
	local expireAtStr = string.sub(tokenStr, colonPos+1)

	-- 调试日志：输出解析结果
	redis.log(redis.LOG_NOTICE, "DEBUG: storedToken = " .. tostring(storedToken))
	redis.log(redis.LOG_NOTICE, "DEBUG: expireAtStr = " .. tostring(expireAtStr))

	-- 2. 验证 Token 匹配性
	if storedToken ~= token then
		redis.log(redis.LOG_NOTICE, "DEBUG: Token mismatch. Expected: " .. token .. ", Got: " .. storedToken)
		return {err="Token mismatch"}
	end

	-- 3. 验证 Token 未过期
	local expireAt = tonumber(expireAtStr)
	if not expireAt or expireAt <= now then
		return {err="Token expired"}
	end

	-- 4. 延长 Token 和 ID 的过期时间
	local newExpireAt = now + lease
	local newTokenStr = token .. ":" .. newExpireAt
	redis.call('HSET', tokenKey, workerID, newTokenStr)
	redis.call('ZADD', key, newExpireAt, workerID)

	return {ok="Success"}
`)

func (g *RedisGenerator) Renew(workerID int64, token string) error {
	if workerID < 1 || workerID > int64(g.maxWorkerID) {
		return ErrInvalidWorkerID
	}
	if len(token) != 22 {
		return ErrInvalidToken
	}

	now, err := g.getCurrentTime()
	if err != nil {
		return fmt.Errorf("get current time failed: %w", err)
	}

	// 添加日志输出
	fmt.Printf("Renew called with workerID: %d, token: %s\n", workerID, token)

	result, err := renewScript.Run(g.ctx, g.redisClient, []string{g.getTokenKey(), g.getIDsKey()},
		workerID, token, now, g.leaseSeconds).Result()
	if err != nil {
		return fmt.Errorf("renew failed: %w", err)
	}

	if result == "Token not found" {
		return ErrNotAssigned
	}
	if result == "Token mismatch" {
		return ErrTokenMismatch
	}
	if result == "Token expired" {
		return ErrTokenExpired
	}

	return nil
}

// Release 主动释放 WorkerID（使其可被重新分配）
func (g *RedisGenerator) Release(workerID int64, token string) error {
	if workerID < 1 || workerID > int64(g.maxWorkerID) {
		return ErrInvalidWorkerID
	}

	if len(token) != 22 {
		return ErrInvalidToken
	}

	key := g.getIDsKey()
	tokenKey := g.getTokenKey()
	now, err := g.getCurrentTime()
	if err != nil {
		return fmt.Errorf("get current time failed: %w", err)
	}

	// 1. 获取 Token 记录（原子操作）
	tokenStr, err := g.redisClient.HGet(g.ctx, tokenKey, strconv.FormatInt(workerID, 10)).Result()
	if err != nil {
		return fmt.Errorf("get token failed: %w", err)
	}
	if tokenStr == "" {
		return ErrNotAssigned
	}

	tokenData := strings.Split(tokenStr, ":")
	if len(tokenData) != 2 {
		return ErrInvalidToken
	}

	// 2. 验证 Token 匹配性
	storedToken := tokenData[0]
	if storedToken != token {
		return ErrTokenMismatch
	}

	// 3. 验证 Token 未过期
	expireAt, err := strconv.ParseInt(tokenData[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expire_at format: %w", err)
	}
	if expireAt <= now {
		return ErrTokenExpired
	}

	// 4. 删除 Token 记录（原子操作）
	_, err = g.redisClient.HDel(g.ctx, tokenKey, strconv.FormatInt(workerID, 10)).Result()
	if err != nil {
		return fmt.Errorf("delete token failed: %w", err)
	}

	// 5. 重置 ID 的过期时间（标记为可用）
	_, err = g.redisClient.ZAdd(g.ctx, key, &redis.Z{
		Score:  0,
		Member: workerID,
	}).Result()
	if err != nil {
		return fmt.Errorf("reset expire time failed: %w", err)
	}

	return nil
}