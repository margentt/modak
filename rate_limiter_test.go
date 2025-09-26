package main

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestBasicRateLimiter(t *testing.T) {
	rl := NewRateLimiterImpl(map[string]RateLimitRule{
		"news": {Limit: 2, Interval: 1 * time.Minute},
	})
	synctest.Test(t, func(t *testing.T) {
		rl.Allow("user1", "news")
		rl.Allow("user1", "news")

		allowed, _ := rl.Allow("user1", "news")
		if allowed {
			t.Fatal("expected rate limit to be exceeded")
		}

		// taking advantage of the new synctest package so we can run time.Sleep without slowing down the test
		time.Sleep(61 * time.Second)

		synctest.Wait()
		allowed, _ = rl.Allow("user1", "news")
		if !allowed {
			t.Fatal("expected rate limit to be reset after interval")
		}
	})
}

func TestMultipleNotificationTypes(t *testing.T) {
	rl := NewRateLimiterImpl(map[string]RateLimitRule{
		"news": {Limit: 2, Interval: 1 * time.Minute},
	})
	synctest.Test(t, func(t *testing.T) {
		rl.Allow("user1", "news")
		rl.Allow("user1", "news")
		rl.Allow("user1", "push")
		rl.Allow("user1", "push")

		allowed, _ := rl.Allow("user1", "news")
		if allowed {
			t.Fatal("expected news rate limit to be exceeded")
		}

		allowed, _ = rl.Allow("user1", "push")
		if !allowed {
			t.Fatal("expected push notifications to be allowed as they have no rate limit")
		}
	})
}

func TestRateLimiterPrune(t *testing.T) {
	rl := NewRateLimiterImpl(map[string]RateLimitRule{
		"news": {Limit: 2, Interval: 1 * time.Minute},
	})
	synctest.Test(t, func(t *testing.T) {
		rl.Allow("user1", "news")
		time.Sleep(59 * time.Second)
		rl.Allow("user1", "news")
		time.Sleep(10 * time.Second)
		rl.Allow("user1", "news")

		allowed, _ := rl.Allow("user1", "news")
		if allowed {
			t.Fatal("expected rate limit to be exceeded")
		}

		timestamps, _ := rl.cache.Get("user1|news")
		if len(timestamps.([]time.Time)) != 2 {
			t.Fatalf("expected 2 timestamps, got %d", len(timestamps.([]time.Time)))
		}
	})
}

func TestConcurrentNotifications(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiterImpl(map[string]RateLimitRule{
		"news": {Limit: 100, Interval: 1 * time.Minute},
	})

	wg := sync.WaitGroup{}
	for range 10000 {
		wg.Go(func() {
			rl.Allow("user1", "news")
		})
	}
	wg.Wait()

	timestamps, _ := rl.cache.Get("user1|news")
	if len(timestamps.([]time.Time)) != 100 {
		t.Fatalf("expected 100 timestamps, got %d", len(timestamps.([]time.Time)))
	}
}

func TestRateLimiterEviction(t *testing.T) {
	rl := NewRateLimiterImpl(map[string]RateLimitRule{
		"news": {Limit: 2, Interval: 1 * time.Minute},
	})
	synctest.Test(t, func(t *testing.T) {
		rl.Allow("user1", "news")
		key := "user1|news"

		_, loaded := rl.mutexes.LoadOrStore(key, &sync.Mutex{})
		if !loaded {
			t.Fatal("expected mutex to be created")
		}

		time.Sleep(3 * time.Minute)
		rl.cache.DeleteExpired()

		_, exists := rl.mutexes.Load(key)
		if exists {
			t.Fatal("expected mutex to be deleted after cache eviction")
		}
	})
}
