package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

func RateLimitWithLuaScript(ctx context.Context, client *redis.Client, bucketKey string, capacity, maxSecondsToWait int, replenishmentRate float64) (allowed bool, timeToWait, tokensRemaining int64, err error) {
	luaScript := redis.NewScript(rateLimitLuaScript)
	sha1, err := luaScript.Load(ctx, client).Result()
	if err != nil {
		return false, 0, 0, err
	}

	result, err := client.EvalSha(ctx, sha1, []string{bucketKey, bucketKey + ":last_replenished"}, capacity, replenishmentRate, time.Now().Unix(), maxSecondsToWait).Result()
	if err != nil {
		return false, 0, 0, err
	}

	response, ok := result.([]interface{})
	if !ok || len(response) != 3 {
		return false, 0, 0, errors.New("unexpected response from Lua script")
	}

	timeToWait, ok = response[1].(int64)
	if !ok {
		return false, 0, 0, errors.New("unexpected response for tokens left")
	}

	tokensRemaining, ok = response[2].(int64)
	if !ok {
		return false, timeToWait, 0, errors.New("unexpected response for tokens left")
	}

	allwd, ok := response[0].(int64)
	if !ok {
		return false, timeToWait, tokensRemaining, errors.New("unexpected response for allowed")
	}

	return allwd == 1, timeToWait, tokensRemaining, nil
}

func MakeMockAPICall() {
	time.Sleep(1 * time.Second)
	return
}

const rateLimitLuaScript = `local bucket_key = KEYS[1]
local last_replenishment_timestamp_key = KEYS[2]
local capacity = tonumber(ARGV[1])
local replenishment_rate = tonumber(ARGV[2])
local current_time = tonumber(ARGV[3])
local max_time_to_wait_for_token = tonumber(ARGV[4])

-- Check if the bucket exists.
local bucket_exists = redis.call('EXISTS', bucket_key)

-- If the bucket does not exist, create the bucket, and set the last replenishment time.
if bucket_exists == 0 then
    redis.call('SET', bucket_key, capacity)
    redis.call('SET', last_replenishment_timestamp_key, current_time)
end

-- Calculate the time difference in seconds since last replenishment
local last_replenishment_timestamp = tonumber(redis.call('GET', last_replenishment_timestamp_key))
local time_difference = current_time - last_replenishment_timestamp
if time_difference < 0 then
	return {-1, 0, 0, time_difference}
end

-- Calculate the amount of tokens to replenish, round down for the number of 'full' tokens.
local num_tokens_to_replenish = math.floor(replenishment_rate * time_difference)

-- Get the current token count in the bucket.
local current_tokens = tonumber(redis.call('GET', bucket_key))

-- Replenish the bucket if there are tokens to replenish
if num_tokens_to_replenish > 0 then
    local available_capacity = capacity - current_tokens
    if available_capacity > 0 then
		-- The number of tokens we add is either the number of tokens we have replenished over
		-- the last time_difference, or enough tokens to refill the bucket completely, whichever
		-- is lower.
        local tokens_to_add = math.min(available_capacity, num_tokens_to_replenish)
        redis.call('INCRBY', bucket_key, tokens_to_add)
        redis.call('SET', last_replenishment_timestamp_key, current_time)
		current_tokens = current_tokens + tokens_to_add
    end
end

local time_to_wait_for_token = 0
-- This is for calculations with us removing a token.
local current_tokens_after_consumption = current_tokens - 1

-- If the bucket will be negative, calculate the needed to 'wait' before using the token.
-- i.e. if we are going to be at -15 tokens after this consumption, and the token replenishment
-- rate is 0.33/s, then we need to wait 45.45 (46 because we round up) seconds before making the request.
if current_tokens_after_consumption < 0 then
    time_to_wait_for_token = math.ceil((current_tokens_after_consumption * -1) / replenishment_rate)
end

if time_to_wait_for_token >= max_time_to_wait_for_token then
    return {0, time_to_wait_for_token, current_tokens}
end

-- Decrement the token bucket by 1, we are granted a token
redis.call('DECRBY', bucket_key, 1)

return {1, time_to_wait_for_token, current_tokens_after_consumption}`