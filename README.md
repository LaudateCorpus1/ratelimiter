ratelimiter
=====

Package ratelimiter implements an LRU based ratelimiter for near-cache type workloads. It's aim is to marry LRU based local caching with Redis's INCR command for incrmenting counters. There are times during data processing where you'd like to be able to say things like "If I see this thing n times in an hour, I don't care about it anymore", or the classic API rate limiting example. Once you hit a certain level of throughput you'd also like to reduce your network hops over to a remote service. 
This package aims to support both kids of workloads. To get the best and most accurate results it's assumed that your keys are partitioned in a way where the same calls go to the same box. The data used in ratelimiter is local to the machine it's on. It's also appromixation rate limiting vs keeping track of any time windows or exact time boundaries. 

The logic is simply.. we'll keep incrementing you, once you pass the max count we're allowing we'll check the time of your key update and compare against the duration difference between now and the rate period you passed in. If it's been longer than that time period we'll reset your count and update the time to give you a fresh set of credits. 

The underlying structure is based on groupcache's LRU library. 

Installation
------------

    go get github.com/CrowdStrike/ratelimiter


Features
--------

* Ability to set a rate limiting time period
* Ability to disable time periods and say once you're ratelimited, you're done
* You can set a maxsize so your memory footprint can remain constant, most used keys stay hot in cache


Example of correct usage where you want to only allow 1000 requests per hour for a given key. Note: If you want to disable lifting the rate period set ratePeriod := 0
That will effectively say for the lifetime of the process if you hit the rate limit you're done. 
```go
  import "github.com/CrowdStrike/ratelimiter"

    maxCapacity := 1000
	ratePeriod := 1 * time.Hour
	rl, err := ratelimiter.New(maxCapacity, ratePeriod)
	if err != nil {
		fmt.Printf("Unable to create cache")
	}

	userKey := "user123"
	maxCount = 100 // the maximum number of items I want from this user in one hour
	cnt, underRateLimit := rl.Incr(userKey, maxCount)
	if underRateLimit {
		// allow further access
		...
	} else {
		fmt.Printf("User [%s] is over rate limit, denying for now, current count [%d]", userKey, cnt)
	}
```

Performance
--------
Approximately 3.2MM Incr operations per second on a standard 2014 macbook pro

```
BenchmarkIncrWithPeriod 5000000       308 ns/op
BenchmarkIncrWithoutPeriod 5000000    253 ns/op
BenchmarkGet10000000                  174 ns/op
```

3.2MM ops / second is more than enough for our needs, but should someone need more we've found adding more selective locking mechanics can be implemented and using the `sync/atomic` package can be used for a ~50% speed up at a minor cost of readability
