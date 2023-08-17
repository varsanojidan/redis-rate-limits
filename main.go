package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/varsanojidan/redis-rate-limits/ratelimit"
)

const (
	bucketKey         = "token_bucket"
	capacity          = 10   //number of tokens in the bucket
	replenishmentRate = 0.33 //tokens per second
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	requestTokensRoutine(client, "A", 10)
	requestTokensRoutine(client, "B", 100)

	for {

	}
}

func requestTokensRoutine(client *redis.Client, name string, maxTimeToWait int) {
	go func() {
		for {
			ctx := context.Background()
			allowed, timeToWait, tokensLeft, err := ratelimit.RateLimitWithLuaScript(ctx, client, bucketKey, capacity, maxTimeToWait, replenishmentRate)
			if err != nil {
				return
			}
			if allowed {
				fmt.Printf("Routine: %s, time to wait: %d, tokens left: %d. got token!\n", name, timeToWait, tokensLeft)
				ratelimit.MakeMockAPICall()
			} else {
				fmt.Printf("Routine: %s, did not got token! Sleeping: time to wait: %d, tokens left: %d, max time to wait: %d\n", name, timeToWait, tokensLeft, maxTimeToWait)
				time.Sleep(5 * time.Second)
			}
		}
	}()
}