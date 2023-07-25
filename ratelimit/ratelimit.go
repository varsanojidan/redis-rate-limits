package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func RateLimitWithLuaScript(ctx context.Context, client *redis.Client, bucketKey string, capacity, refillRate, requestTokens int, lock bool) (allowed, locked bool, tokensLeft int, err error) {
	luaScript := redis.NewScript(rateLimitLuaScript)
	sha1, err := luaScript.Load(ctx, client).Result()
	if err != nil {
		return false, false, 0, err
	}

	lockKey := ""
	if lock {
		lockKey = bucketKey + ":lock"
	}
	result, err := client.EvalSha(ctx, sha1, []string{bucketKey}, capacity, refillRate, requestTokens, lockKey).Result()
	if err != nil {
		return false, false, 0, err
	}

	response, ok := result.([]interface{})
	if !ok || len(response) != 2 {
		return false, false, 0, errors.New("unexpected response from Lua script")
	}

	allwd, ok := response[0].(int64)
	if !ok {
		return false, false, 0, errors.New("unexpected response for allowed")
	}

	// locked
	if allwd == -1 {
		return false, true, 0, nil
	}

	remTokens, ok := response[1].(int64)
	if !ok {
		return false, false, 0, errors.New("unexpected response for tokens left")
	}

	return allwd == 1, false, int(remTokens), nil
}

func MakeMockAPICallAndUnlockBucket(ctx context.Context, client *redis.Client, lockKey string) (bool, error) {
	time.Sleep(1 * time.Second)
	err := client.Del(ctx, lockKey).Err()
	if err != nil {
		fmt.Println("failed to release lock")
	}
	fmt.Println("released lock")
	return true, nil
}

const rateLimitLuaScript = `local bucket_key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local request_tokens = tonumber(ARGV[3])
local mutex_key = ARGV[4] or ''

-- If the mutex key is provided and exists, reject the request.
if mutex_key ~= '' and redis.call('EXISTS', mutex_key) == 1 then
    return {-1, -1}
end

-- If the mutex key is provided, set the mutex to lock the bucket and set its expiration time to 30 seconds.
if mutex_key ~= '' then
    redis.call('SET', mutex_key, 1, 'EX', 30)
end

-- Check if the bucket exists.
local bucket_exists = redis.call('EXISTS', bucket_key)

-- If the bucket does not exist or has expired, replenish the bucket and set the new expiration time.
if bucket_exists == 0 then
    redis.call('SET', bucket_key, capacity)
    redis.call('EXPIRE', bucket_key, 30)
end

-- Get the current token count in the bucket.
local current_tokens = tonumber(redis.call('GET', bucket_key))

-- Check if there are enough tokens for the current request.
local allowed = (current_tokens >= request_tokens)

-- If the request is allowed, decrement the tokens for this request.
if allowed then
    redis.call('DECRBY', bucket_key, request_tokens)
end

return {allowed and 1 or 0, tonumber(redis.call('GET', bucket_key))}`