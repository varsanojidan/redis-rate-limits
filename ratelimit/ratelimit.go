package ratelimit

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

func RateLimitWithLuaScript(ctx context.Context, client *redis.Client, bucketKey string, capacity, refillRate, requestTokens int) (bool, int, error) {
	luaScript := redis.NewScript(rateLimitLuaScript)
	sha1, err := luaScript.Load(ctx, client).Result()
	if err != nil {
		return false, 0, err
	}

	result, err := client.EvalSha(ctx, sha1, []string{bucketKey}, capacity, refillRate, requestTokens).Result()
	if err != nil {
		return false, 0, err
	}

	response, ok := result.([]interface{})
	if !ok || len(response) != 2 {
		return false, 0, errors.New("unexpected response from Lua script")
	}

	allowed, ok := response[0].(int64)
	if !ok {
		return false, 0, errors.New("unexpected response for allowed")
	}

	tokensLeft, ok := response[1].(int64)
	if !ok {
		return false, 0, errors.New("unexpected response for tokens left")
	}

	return allowed == 1, int(tokensLeft), nil
}

const rateLimitLuaScript = `local bucket_key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local request_tokens = tonumber(ARGV[3])

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