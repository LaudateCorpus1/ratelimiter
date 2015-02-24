package ratelimiter

import (
	"fmt"
	"testing"
	"time"
)

func TestEmptyCacheErrors(t *testing.T) {
	_, err := New(0, 100*time.Second)
	if err == nil {
		t.Fatalf("expected a maxentry size of 0 would fail Cache creation")
	}
}

func TestIncr(t *testing.T) {
	rl, err := New(100, 2*time.Second)
	if err != nil {
		t.Fatalf("Cache should have been created OK")
	}

	key := "foo"
	maxCount := 100
	cnt, ok := rl.Incr(key, maxCount)
	if cnt != 1 {
		t.Fatalf("count should have been [1] actual [%d]", cnt)
	}
	if !ok {
		t.Fatalf("expected a brand new key would not be rate limited")
	}

	cnt, ok = rl.Incr(key, maxCount)
	if cnt != 2 {
		t.Fatalf("count should have been [2] actual [%d]", cnt)
	}

}

// expected that incrementing a key 100 times within 10 seconds
// returns a false for the ratelimiting status
func TestBasicRateLimiting(t *testing.T) {

	rl, _ := New(100, 10*time.Second)

	maxCount := 10
	key := "foo"
	for i := 0; i < 15; i++ {
		cnt, underRateLimit := rl.Incr(key, maxCount)
		if int(cnt) > maxCount && underRateLimit {
			t.Fatalf("expected that if we went over [%d] increments ratelimit would be false, but was true", maxCount)
		}

	}
}

func TestMaxItemsInCache(t *testing.T) {
	maxItemsInCache := 10
	rl, _ := New(maxItemsInCache, 10*time.Second)

	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("foo_%d", i)
		_, _ = rl.Incr(key, 1)
	}

	if rl.Len() > maxItemsInCache {
		t.Fatalf("expected to only have [%d] items in cache, actually got [%d]", maxItemsInCache, rl.Len())
	}

}

func TestGet(t *testing.T) {
	maxItemsInCache := 10
	rl, _ := New(maxItemsInCache, 10*time.Second)

	key := "foo"
	_, _ = rl.Incr(key, 10)

	cnt, _ := rl.Get(key)
	if cnt != 1 {
		t.Fatalf("expected to get foo with a count of [1] but got [%d]", cnt)
	}

	_, _ = rl.Incr(key, 10)

	cnt, _ = rl.Get(key)
	if cnt != 2 {
		t.Fatalf("expected to get foo with a count of [2] but got [%d]", cnt)
	}

}

func TestRemove(t *testing.T) {
	maxItemsInCache := 10
	rl, _ := New(maxItemsInCache, 10*time.Second)

	key := "foo"
	_, _ = rl.Incr(key, 10)

	cnt, _ := rl.Get(key)
	if cnt != 1 {
		t.Fatalf("expected to get foo with a count of [1] but got [%d]", cnt)
	}

	rl.Remove(key)

	_, ok := rl.Get(key)
	if ok {
		t.Fatalf("should have gotten false back since I deleted the key")
	}

}

// by setting a 0 second duration we're saying we don't want to ever clear the rate limit
// once you're rate limited you're done
func TestRateLimitDoesntRemove(t *testing.T) {

	rl, _ := New(100, 0)

	maxCount := 10
	key := "foo"

	_, underRateLimit := rl.Incr(key, maxCount)

	if !underRateLimit {
		t.Fatalf("expected for a new key that incrementing the first time would keep us under the ratelimit")
	}

	// push us over the rate limit
	for i := 0; i < 15; i++ {
		_, _ = rl.Incr(key, maxCount)
	}

	_, underRateLimit = rl.Incr(key, maxCount)

	if underRateLimit {
		t.Fatalf("expected that if we went over [%d] increments ratelimit would be true, but was false", maxCount)
	}

	// sleep for 3 seconds and we should be OK again
	time.Sleep(3 * time.Second)
	cnt, underRateLimit := rl.Incr(key, maxCount)

	if underRateLimit {
		t.Fatalf("expected that since we don't went to apply a rate limit we'd still be rate limited [%d]", cnt)
	}

}

// ensure that after n seconds our rate limit no longer applies
func TestRateLimitGetsRemoved(t *testing.T) {

	rl, _ := New(100, 2*time.Second)

	maxCount := 10
	key := "foo"

	_, underRateLimit := rl.Incr(key, maxCount)

	if !underRateLimit {
		t.Fatalf("expected for a new key that incrementing the first time would keep us under the ratelimit")
	}

	// push us over the rate limit
	for i := 0; i < 15; i++ {
		_, _ = rl.Incr(key, maxCount)
	}

	_, underRateLimit = rl.Incr(key, maxCount)

	if underRateLimit {
		t.Fatalf("expected that if we went over [%d] increments ratelimit would be true, but was false", maxCount)
	}

	// sleep for 3 seconds and we should be OK again
	time.Sleep(3 * time.Second)
	cnt, underRateLimit := rl.Incr(key, maxCount)

	if !underRateLimit {
		t.Fatalf("expected that if we slept for a while to pass the ttl that we'd be ok again but our count was [%d]", cnt)
	}

	for i := 0; i < 15; i++ {
		_, _ = rl.Incr(key, maxCount)
	}

	_, underRateLimit = rl.Incr(key, maxCount)

	if underRateLimit {
		t.Fatalf("expected that if we went over [%d] increments ratelimit would be true, but was false", maxCount)
	}

	// sleep for 3 seconds and we should be OK again
	time.Sleep(3 * time.Second)
	cnt, underRateLimit = rl.Incr(key, maxCount)
	if !underRateLimit {
		t.Fatalf("expected that if we slept for a while to pass the ttl that we'd be ok again but our count was [%d]", cnt)
	}

}

func TestOnEvictedCallback(t *testing.T) {

	keys := []string{"foo", "bar", "baz"}

	// We will only allow max items of 2, but will incr 3, so the first one in "foo" will be evicted and we should be notified
	callback := func(key interface{}, value interface{}) {
		if key.(string) != keys[0] {
			t.Fatalf("Expected %s to be purged and sent in callback, got %s instead", keys[0], key.(string))
		}
	}

	maxItemsInCache := 2
	rl, _ := New(maxItemsInCache, 10*time.Second)
	rl.OnEvicted = callback

	for _, key := range keys {
		_, _ = rl.Incr(key, 10)
	}

	// The other two keys should still be present with a count of 1
	for _, key := range keys[1:] {
		cnt, _ := rl.Get(key)
		if cnt != 1 {
			t.Fatalf("expected to get %s with a count of [1] but got [%d]", key, cnt)
		}
	}

}

// BENCHMARKS
// go test -bench=. -run=XXX
// on macbook pro ~2.7 million ops a second
func BenchmarkIncrWithPeriod(b *testing.B) {
	rl, _ := New(100, 2*time.Second)
	maxCount := 10
	key := "foo"
	for n := 0; n < b.N; n++ {
		rl.Incr(key, maxCount)
	}
}

// on macbook pro ~3 million ops a second
func BenchmarkIncrWithoutPeriod(b *testing.B) {
	rl, _ := New(100, 0)
	maxCount := 10
	key := "foo"
	for n := 0; n < b.N; n++ {
		rl.Incr(key, maxCount)
	}
}

func BenchmarkGet(b *testing.B) {
	rl, _ := New(100, 2*time.Second)
	key := "foo"
	for n := 0; n < b.N; n++ {
		_, _ = rl.Get(key)
	}
}
