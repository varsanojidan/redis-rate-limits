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
	capacity               = 10
	refillRatePer30Seconds = 10
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	go func() {
		for {
			allowed, err := requestTokens(client, 2)
			if err != nil {
				fmt.Println("GoRoutine A Error:", err)
				time.Sleep(5 * time.Second)
			}
			if allowed {
				fmt.Println("GoRoutine A Request processed successfully.")
			} else {
				fmt.Println("GoRoutine A Rate limit exceeded. Please try again later.")
			}
		}
	}()
	go func() {
		for {
			allowed, err := requestTokens(client, 1)
			if err != nil {
				fmt.Println("GoRoutine B Error:", err)
				time.Sleep(5 * time.Second)
			}
			if allowed {
				fmt.Println("GoRoutine B Request processed successfully.")
			} else {
				fmt.Println("GoRoutine B Rate limit exceeded. Please try again later.")
			}
		}
	}()

	for {

	}
}

func requestTokens(client *redis.Client, tokensWanted int) (bool, error) {
	return ratelimit.RateLimitWithLuaScript(context.Background(), client, bucketKey, capacity, refillRatePer30Seconds, tokensWanted)
}