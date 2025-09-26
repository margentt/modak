package main

import (
	"sort"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type RateLimitRule struct {
	Limit    int           // max number of notifications
	Interval time.Duration // per interval
}

type RateLimiter interface {
	Allow(recipient, notificationType string) (bool, error)
}

// RateLimiterImpl implements the RateLimiter interface using an in-memory cache
type RateLimiterImpl struct {
	cache *gocache.Cache
	// mutexes for each recipient/notificationType pair
	mutexes sync.Map // map[string]*sync.Mutex
	// using a map instead of a slice for O(1) lookups but also to prevent multiple rules for the same notification type
	Rules map[string]RateLimitRule
}

func NewRateLimiterImpl(rules map[string]RateLimitRule) *RateLimiterImpl {
	rateLimiter := &RateLimiterImpl{
		// the cleanup interval doesn't have to be so frequent since we always prune the old timestamps on each Allow call
		// the main purpose of the cleanup is to free up memory from expired entries
		cache:   gocache.New(-1, 2*time.Minute),
		Rules:   rules,
		mutexes: sync.Map{},
	}

	// remove mutex when cache entry is evicted
	rateLimiter.cache.OnEvicted(func(k string, v any) {
		rateLimiter.mutexes.Delete(k)
	})

	return rateLimiter
}

func (r *RateLimiterImpl) Allow(recipient, notificationType string) (bool, error) {
	rule, ok := r.Rules[notificationType]
	if !ok {
		return true, nil
	}

	key := recipient + "|" + notificationType

	muAny, _ := r.mutexes.LoadOrStore(key, &sync.Mutex{})
	mu := muAny.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	var timestamps []time.Time
	if v, ok := r.cache.Get(key); ok {
		timestamps = v.([]time.Time)
	} else {
		timestamps = []time.Time{}
	}

	cutoff := time.Now().Add(-rule.Interval)
	// perform binary search to find the cutoff index
	cutoffIdx := sort.Search(len(timestamps), func(i int) bool {
		return timestamps[i].After(cutoff)
	})

	// prune old timestamps
	if cutoffIdx > 0 {
		// create a new slice instead of slicing the original one to avoid memory leaks
		// as the backing array would still hold references to old timestamps
		timestamps = append([]time.Time(nil), timestamps[cutoffIdx:]...)
	}

	// check if limit was exceeded
	if len(timestamps) >= rule.Limit {
		// we still have to update the TTL regardless of whether we allow or deny
		r.cache.Set(key, timestamps, rule.Interval)
		return false, nil
	}

	// append new timestamp and update cache
	timestamps = append(timestamps, time.Now())
	r.cache.Set(key, timestamps, rule.Interval)
	return true, nil
}
