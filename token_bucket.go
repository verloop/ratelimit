package ratelimit

import (
	"fmt"
	"time"
)

var timeNow = time.Now

type Bucket struct {
	Identifier string
	Rate       int
	WindowSize int
	Store      Store
}

type Status struct {
	Allowed     bool
	Remaining   int
	NextRefresh time.Duration
}

type StoreResponse struct {
	Allowed    bool
	Counter    int
	LastRefill time.Time
}

// The token bucket algorithm
// given a key like `user-post:avinassh`, rate
// if the doesn't exist then allow, refill and allow
// if key exists
// 		- check if the current value less than limit, if less allow and increment
//		- if the limit has already exceeded, then see if it can be refilled

// NewTokenBucket returns a new rate limiter which uses the token bucket algorithm.
func NewTokenBucket(identifier string, rate, windowSize int, store Store) Bucket {
	return Bucket{identifier, rate, windowSize, store}
}

func (b *Bucket) Allow(key string) (bool, error) {
	s, err := b.AllowWithStatus(key)
	if err != nil {
		return false, err
	}
	return s.Allowed, nil
}

func (b *Bucket) AllowWithStatus(key string) (Status, error) {
	userKey := fmt.Sprintf("%s:%s", b.Identifier, key)
	res, err := b.Store.Inc(userKey, b.Rate, b.WindowSize, int(TimeMillis(timeNow())))
	if err != nil {
		return Status{}, err
	}
	timeElapsed := timeNow().Sub(res.LastRefill)
	s := Status{
		Allowed:     res.Allowed,
		Remaining:   b.Rate - res.Counter,
		NextRefresh: timeElapsed + (time.Duration(b.WindowSize) * time.Millisecond),
	}
	return s, nil
}
