package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/varsanojidan/redis-rate-limits/ratelimit"
)

const (
	bucketKey              = "token_bucket"
	capacity               = 20
	refillRatePer30Seconds = 20
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	requestTokensRoutine(client, "A", 2)
	requestTokensRoutine(client, "B", 3)

	for {

	}
}

func requestTokensRoutine(client *redis.Client, name string, tokensWanted int) {
	go func() {
		for {
			allowed, left, err := ratelimit.RateLimitWithLuaScript(context.Background(), client, bucketKey, capacity, refillRatePer30Seconds, tokensWanted)
			if err != nil {
				fmt.Printf("Routine: %+v, encountered error: %+v\n", name, err)
				return
			}
			if allowed {
				fmt.Printf("Routine: %s, remaining tokens: %d, got token!\n", name, left)
			} else {
				fmt.Printf("Routine: %s, failed to get token!\n", name)
				time.Sleep(5 * time.Second)
			}
		}
	}()
}