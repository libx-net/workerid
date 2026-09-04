package workerid

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RedisDoer is a driver-agnostic Redis command executor.
// Users can adapt go-redis v7/v8/v9 (or any other client) with a small closure.
type RedisDoer interface {
	Do(ctx context.Context, args ...any) (any, error)
}

// RedisFunc lets any function implement RedisDoer.
type RedisFunc func(ctx context.Context, args ...any) (any, error)

func (f RedisFunc) Do(ctx context.Context, args ...any) (any, error) {
	return f(ctx, args...)
}

type RedisGenerator struct {
	cluster      string
	maxWorkerID  uint32
	leaseSeconds int
	redisClient  RedisDoer
	ctx          context.Context
	clockSync    bool
	lockKey      string
	lockVal      string
}

var _ Generator = (*RedisGenerator)(nil)

// NewRedisGenerator creates a RedisGenerator instance.
// client must implement RedisDoer; adapt go-redis with RedisFunc, e.g.:
//
//	doer := workerid.RedisFunc(func(ctx context.Context, args ...any) (any, error) {
//		return client.Do(ctx, args...).Result()
//	})
func NewRedisGenerator(redisClient RedisDoer, cluster string, options ...Option) (*RedisGenerator, error) {
	if redisClient == nil {
		return nil, errors.New("redis client is nil")
	}
	cfg, err := ResolveConfig(cluster, options...)
	if err != nil {
		return nil, err
	}

	allocator := &RedisGenerator{
		cluster:      cfg.Cluster,
		maxWorkerID:  cfg.MaxWorkerID,
		leaseSeconds: int(cfg.MaxLeaseTime.Seconds()),
		redisClient:  redisClient,
		ctx:          context.Background(),
		lockKey:      fmt.Sprintf("{workerid:cluster:%s}:lock", cfg.Cluster),
		lockVal:      generateToken(),
	}

	if err := allocator.initAvailableIDs(); err != nil {
		return nil, fmt.Errorf("initialize available IDs failed: %w", err)
	}

	return allocator, nil
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case string:
		return strconv.ParseInt(x, 10, 64)
	case []byte:
		return strconv.ParseInt(string(x), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected type %T", v)
	}
}

func toString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []byte:
		return string(x), nil
	default:
		return "", fmt.Errorf("unexpected type %T", v)
	}
}

func (g *RedisGenerator) eval(script string, keys []string, args ...any) (any, error) {
	cmdArgs := make([]any, 0, 3+len(keys)+len(args))
	cmdArgs = append(cmdArgs, "EVAL", script, len(keys))
	for _, k := range keys {
		cmdArgs = append(cmdArgs, k)
	}
	cmdArgs = append(cmdArgs, args...)
	return g.redisClient.Do(g.ctx, cmdArgs...)
}

func (g *RedisGenerator) getCurrentTime() (int64, error) {
	if g.clockSync {
		reply, err := g.redisClient.Do(g.ctx, "TIME")
		if err != nil {
			return 0, err
		}
		arr, ok := reply.([]any)
		if !ok || len(arr) == 0 {
			return 0, fmt.Errorf("unexpected TIME reply type %T", reply)
		}
		sec, err := toInt64(arr[0])
		if err != nil {
			return 0, fmt.Errorf("parse TIME reply: %w", err)
		}
		return sec, nil
	}
	return time.Now().Unix(), nil
}

// initIDsScript persists max_worker_id next to the zset (same hash tag) and
// seeds 0..maxID when the zset is empty. Returns 1 if seeded, 0 if the zset
// already existed with a matching max, or "MISMATCH:<stored>" on mismatch.
var initIDsScript = `
	local idsKey = KEYS[1]
	local metaKey = KEYS[2]
	local maxID = tonumber(ARGV[1])

	redis.call('SETNX', metaKey, maxID)
	local stored = tonumber(redis.call('GET', metaKey))
	if stored ~= maxID then
		return 'MISMATCH:' .. tostring(stored)
	end
	if redis.call('ZCARD', idsKey) > 0 then
		return 0
	end
	for i = 0, maxID do
		redis.call('ZADD', idsKey, 0, tostring(i))
	end
	return 1
`

func (g *RedisGenerator) initAvailableIDs() error {
	reply, err := g.eval(initIDsScript, []string{g.getIDsKey(), g.getMaxWorkerIDKey()}, g.maxWorkerID)
	if err != nil {
		return err
	}
	return initReplyError(reply, g.maxWorkerID)
}

func initReplyError(reply any, configured uint32) error {
	s, err := toString(reply)
	if err != nil {
		return nil
	}
	const prefix = "MISMATCH:"
	if !strings.HasPrefix(s, prefix) {
		return nil
	}
	stored, err := toInt64(s[len(prefix):])
	if err != nil {
		return fmt.Errorf("%w: configured=%d", ErrClusterConfigMismatch, configured)
	}
	return fmt.Errorf("%w: stored=%d configured=%d", ErrClusterConfigMismatch, stored, configured)
}

// getIDsKey returns the Sorted Set key for WorkerIDs
func (g *RedisGenerator) getIDsKey() string {
	return fmt.Sprintf("{workerid:cluster:%s}:ids", g.cluster)
}

// getMaxWorkerIDKey returns the string key that stores this cluster's max_worker_id.
func (g *RedisGenerator) getMaxWorkerIDKey() string {
	return fmt.Sprintf("{workerid:cluster:%s}:max_worker_id", g.cluster)
}

// getTokenKey returns the Token storage key
func (g *RedisGenerator) getTokenKey() string {
	return fmt.Sprintf("{workerid:cluster:%s}:tokens", g.cluster)
}

var getIDScript = `
	local key = KEYS[1]
	local now = tonumber(ARGV[1])
	local lease = tonumber(ARGV[2])

	-- Find the smallest available ID
	local ids = redis.call('ZRANGEBYSCORE', key, '-inf', now, 'WITHSCORES', 'LIMIT', 0, 1)
	if #ids == 0 then return -1 end

	local workerID = ids[1]
	local newExpire = now + lease

	-- Update ID state
	redis.call('ZADD', key, newExpire, workerID)

	-- Store Token
	local tokenKey = KEYS[2]
	local token = ARGV[3]
	local tokenData = token .. ':' .. newExpire
	redis.call('HSET', tokenKey, workerID, tokenData)
	redis.call('EXPIRE', tokenKey, lease * 3)  -- Set Token hash expiration

	return tonumber(workerID)
`

func (g *RedisGenerator) GetID() (int64, string, error) {
	token := generateToken()
	now, err := g.getCurrentTime()
	if err != nil {
		return 0, "", fmt.Errorf("get current time failed: %w", err)
	}
	reply, err := g.eval(getIDScript, []string{g.getIDsKey(), g.getTokenKey()},
		now, g.leaseSeconds, token)
	if err != nil {
		return 0, "", fmt.Errorf("get ID failed: %w", err)
	}
	result, err := toInt64(reply)
	if err != nil {
		return 0, "", fmt.Errorf("get ID failed: %w", err)
	}
	if result < 0 {
		return 0, "", ErrNoAvailableID
	}
	return result, token, nil
}

var renewScript = `
	local tokenKey = KEYS[1]
	local key = KEYS[2]
	local workerID = ARGV[1]
	local token = ARGV[2]
	local now = tonumber(ARGV[3])
	local lease = tonumber(ARGV[4])

	-- 1. Get Token record
	local tokenStr = redis.call('HGET', tokenKey, workerID)
	if not tokenStr then
		return 'NOT_FOUND'
	end

	-- More reliable string split
	local colonPos = string.find(tokenStr, ":")
	if not colonPos then
		return 'INVALID'
	end
	local storedToken = string.sub(tokenStr, 1, colonPos-1)
	local expireAtStr = string.sub(tokenStr, colonPos+1)

	-- 2. Verify Token match
	if storedToken ~= token then
		return 'MISMATCH'
	end

	-- 3. Verify Token not expired
	local expireAt = tonumber(expireAtStr)
	if not expireAt or expireAt <= now then
		return 'EXPIRED'
	end

	-- 4. Extend Token and ID expiration
	local newExpireAt = now + lease
	local newTokenStr = token .. ":" .. newExpireAt
	redis.call('HSET', tokenKey, workerID, newTokenStr)
	redis.call('ZADD', key, newExpireAt, workerID)

	-- 5. Refresh Token Hash TTL to prevent whole Hash expiration
	redis.call('EXPIRE', tokenKey, lease * 3)

	return 'OK'
`

func (g *RedisGenerator) mapTokenStatus(status string) error {
	switch status {
	case "OK":
		return nil
	case "NOT_FOUND":
		return ErrNotAssigned
	case "MISMATCH":
		return ErrTokenMismatch
	case "EXPIRED":
		return ErrTokenExpired
	case "INVALID":
		return ErrInvalidToken
	default:
		return fmt.Errorf("unexpected status: %s", status)
	}
}

func (g *RedisGenerator) Renew(workerID int64, token string) error {
	if workerID < 0 || workerID > int64(g.maxWorkerID) {
		return ErrInvalidWorkerID
	}
	if len(token) != 22 {
		return ErrInvalidToken
	}

	now, err := g.getCurrentTime()
	if err != nil {
		return fmt.Errorf("get current time failed: %w", err)
	}

	reply, err := g.eval(renewScript, []string{g.getTokenKey(), g.getIDsKey()},
		workerID, token, now, g.leaseSeconds)
	if err != nil {
		return fmt.Errorf("renew failed: %w", err)
	}

	status, err := toString(reply)
	if err != nil {
		return fmt.Errorf("renew failed: %w", err)
	}
	return g.mapTokenStatus(status)
}

var releaseScript = `
	local tokenKey = KEYS[1]
	local key = KEYS[2]
	local workerID = ARGV[1]
	local token = ARGV[2]
	local now = tonumber(ARGV[3])

	local tokenStr = redis.call('HGET', tokenKey, workerID)
	if not tokenStr then
		return 'NOT_FOUND'
	end

	local colonPos = string.find(tokenStr, ":")
	if not colonPos then
		return 'INVALID'
	end
	local storedToken = string.sub(tokenStr, 1, colonPos-1)
	local expireAtStr = string.sub(tokenStr, colonPos+1)

	if storedToken ~= token then
		return 'MISMATCH'
	end

	local expireAt = tonumber(expireAtStr)
	if not expireAt or expireAt <= now then
		return 'EXPIRED'
	end

	redis.call('HDEL', tokenKey, workerID)
	redis.call('ZADD', key, 0, workerID)

	return 'OK'
`

// Release actively releases a WorkerID (making it available for reallocation)
func (g *RedisGenerator) Release(workerID int64, token string) error {
	if workerID < 0 || workerID > int64(g.maxWorkerID) {
		return ErrInvalidWorkerID
	}

	if len(token) != 22 {
		return ErrInvalidToken
	}

	now, err := g.getCurrentTime()
	if err != nil {
		return fmt.Errorf("get current time failed: %w", err)
	}

	reply, err := g.eval(releaseScript, []string{g.getTokenKey(), g.getIDsKey()},
		workerID, token, now)
	if err != nil {
		return fmt.Errorf("release failed: %w", err)
	}

	status, err := toString(reply)
	if err != nil {
		return fmt.Errorf("release failed: %w", err)
	}
	return g.mapTokenStatus(status)
}
