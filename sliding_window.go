package ratelimit

import (
	"fmt"
	"time"
)

type SlidingWindow struct {
	Identifier string
	Rate       int
	WindowSize int
	Store      Store
}

// The sliding window algorithm
// given a key like `user-post:avinassh`, rate
// if the doesn't exist then allow, refill and allow
// if key exists
// 		- check if the current value less than limit, if less allow and increment
//		- if the limit has already exceeded, then see if it can be refilled

// NewSlidingWindow returns a new rate limiter which uses the token bucket algorithm.
func NewSlidingWindow(identifier string, rate, windowSize int, store Store) SlidingWindow {
	return SlidingWindow{identifier, rate, windowSize, store}
}

func (sw *SlidingWindow) Allow(key string) (bool, error) {
	s, err := sw.AllowWithStatus(key)
	if err != nil {
		return false, err
	}
	return s.Allowed, nil
}

func (sw *SlidingWindow) AllowWithStatus(key string) (Status, error) {
	userKey := fmt.Sprintf("%s:%s", sw.Identifier, key)
	res, err := sw.Store.Inc(userKey, sw.Rate, sw.WindowSize, int(TimeMillis(timeNow())))
	if err != nil {
		return Status{}, err
	}
	timeElapsed := timeNow().Sub(res.LastRefill)
	s := Status{
		Allowed:     res.Allowed,
		Remaining:   sw.Rate - res.Counter,
		NextRefresh: timeElapsed + (time.Duration(sw.WindowSize) * time.Millisecond),
	}
	return s, nil
}
