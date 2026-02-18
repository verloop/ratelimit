package ratelimit

import (
	"testing"
)

func TestRedigoTBStore_inc(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)

	// let's start with a key which doesn't exist yet
	key := "test"
	now := 1000
	limit := 5
	windowSize := 1000 // i.e. 1s in millis
	success := 1
	fail := 0

	// clean up
	defer func() {
		pool.Get().Do("DEL", key)
	}()

	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != now {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], now)
		}
		if res["c"] != i {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], i)
		}
		if res["s"] != success {
			t.Errorf("invalid status got = %d, want = %d", res["s"], success)
		}
	}

	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != now {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], now)
		}
		if res["c"] != limit {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], i)
		}
		if res["s"] != fail {
			t.Errorf("invalid status got = %d, want = %d", res["s"], fail)
		}
	}

	// lets increase the timestamp by 1000 and it should still fail
	expectedNow := now
	now = 1999
	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != expectedNow {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], expectedNow)
		}
		if res["c"] != limit {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], limit)
		}
		if res["s"] != fail {
			t.Errorf("invalid status got = %d, want = %d", res["s"], fail)
		}
	}

	// lets elapse a second and now it should pass again
	now = 2000
	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != now {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], now)
		}
		if res["c"] != i {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], i)
		}
		if res["s"] != success {
			t.Errorf("invalid status got = %d, want = %d", res["s"], success)
		}
	}
}

func TestRedigoTBStore_Rate10(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)

	key := "test_rate10"
	now := 1000
	limit := 10
	windowSize := 1000
	success := 1
	fail := 0

	defer func() {
		pool.Get().Do("DEL", key)
	}()

	// 10 requests should all succeed
	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Fatalf("inc call %d failed: %v", i, err)
		}
		if res["s"] != success {
			t.Errorf("request %d: expected success, got status=%d", i, res["s"])
		}
		if res["c"] != i {
			t.Errorf("request %d: expected counter=%d, got=%d", i, i, res["c"])
		}
	}

	// 11th request should be denied
	res, err := store.inc(key, limit, windowSize, now)
	if err != nil {
		t.Fatalf("inc call 11 failed: %v", err)
	}
	if res["s"] != fail {
		t.Errorf("request 11: expected denial, got status=%d", res["s"])
	}
	if res["c"] != limit {
		t.Errorf("request 11: expected counter=%d, got=%d", limit, res["c"])
	}

	// after window reset, requests should succeed again
	now = 2000
	res, err = store.inc(key, limit, windowSize, now)
	if err != nil {
		t.Fatalf("inc after reset failed: %v", err)
	}
	if res["s"] != success {
		t.Errorf("after reset: expected success, got status=%d", res["s"])
	}
	if res["c"] != 1 {
		t.Errorf("after reset: expected counter=1, got=%d", res["c"])
	}
}

func TestRedigoTBStore_BurstPattern(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)

	key := "test_burst"
	now := 1000
	limit := 10
	windowSize := 1000
	success := 1
	fail := 0

	defer func() {
		pool.Get().Do("DEL", key)
	}()

	// burst: 10 requests at same timestamp T
	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Fatalf("burst request %d failed: %v", i, err)
		}
		if res["s"] != success {
			t.Errorf("burst request %d: expected success, got status=%d", i, res["s"])
		}
	}

	// 11th at same timestamp should be denied
	res, err := store.inc(key, limit, windowSize, now)
	if err != nil {
		t.Fatalf("burst request 11 failed: %v", err)
	}
	if res["s"] != fail {
		t.Errorf("burst request 11: expected denial, got status=%d", res["s"])
	}

	// T+999 should still be denied (within same window)
	res, err = store.inc(key, limit, windowSize, now+999)
	if err != nil {
		t.Fatalf("request at T+999 failed: %v", err)
	}
	if res["s"] != fail {
		t.Errorf("at T+999: expected denial, got status=%d", res["s"])
	}

	// T+1000 should succeed (new window)
	res, err = store.inc(key, limit, windowSize, now+1000)
	if err != nil {
		t.Fatalf("request at T+1000 failed: %v", err)
	}
	if res["s"] != success {
		t.Errorf("at T+1000: expected success, got status=%d", res["s"])
	}
}

func TestRedigoTBStore_WindowBoundaryPrecision(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)

	key := "test_boundary"
	limit := 10
	windowSize := 1000
	success := 1
	fail := 0
	lastRefill := 5000

	defer func() {
		pool.Get().Do("DEL", key)
	}()

	// fill the bucket at lastRefill
	for i := 1; i <= limit; i++ {
		_, err := store.inc(key, limit, windowSize, lastRefill)
		if err != nil {
			t.Fatalf("fill request %d failed: %v", i, err)
		}
	}

	// lastRefill+999 should be denied (window hasn't elapsed)
	res, err := store.inc(key, limit, windowSize, lastRefill+999)
	if err != nil {
		t.Fatalf("request at lastRefill+999 failed: %v", err)
	}
	if res["s"] != fail {
		t.Errorf("at lastRefill+999: expected denial, got status=%d", res["s"])
	}

	// lastRefill+1000 should be allowed (window has elapsed)
	res, err = store.inc(key, limit, windowSize, lastRefill+1000)
	if err != nil {
		t.Fatalf("request at lastRefill+1000 failed: %v", err)
	}
	if res["s"] != success {
		t.Errorf("at lastRefill+1000: expected success, got status=%d", res["s"])
	}
	if res["c"] != 1 {
		t.Errorf("at lastRefill+1000: expected counter=1, got=%d", res["c"])
	}
	if res["ts"] != lastRefill+1000 {
		t.Errorf("at lastRefill+1000: expected ts=%d, got=%d", lastRefill+1000, res["ts"])
	}
}

func TestRedigoTBStore_CounterAccuracy(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)

	key := "test_counter"
	limit := 10
	windowSize := 1000
	now := 1000

	defer func() {
		pool.Get().Do("DEL", key)
	}()

	// verify counter increments exactly
	for i := 1; i <= limit+5; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if i <= limit {
			if res["c"] != i {
				t.Errorf("request %d: expected counter=%d, got=%d", i, i, res["c"])
			}
		} else {
			// after limit, counter should stay at limit
			if res["c"] != limit {
				t.Errorf("request %d: expected counter=%d (capped), got=%d", i, limit, res["c"])
			}
		}
	}
}

func TestRedigoTBStore_CustomWindow(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)

	key := "test_custom_window"
	limit := 5
	windowSize := 500 // 500ms window instead of default 1000ms
	success := 1
	fail := 0
	lastRefill := 5000

	defer func() {
		pool.Get().Do("DEL", key)
	}()

	// fill the bucket at lastRefill
	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, lastRefill)
		if err != nil {
			t.Fatalf("fill request %d failed: %v", i, err)
		}
		if res["s"] != success {
			t.Errorf("fill request %d: expected success, got status=%d", i, res["s"])
		}
	}

	// at lastRefill+499 should be denied (window hasn't elapsed)
	res, err := store.inc(key, limit, windowSize, lastRefill+499)
	if err != nil {
		t.Fatalf("request at lastRefill+499 failed: %v", err)
	}
	if res["s"] != fail {
		t.Errorf("at lastRefill+499: expected denial, got status=%d", res["s"])
	}

	// at lastRefill+500 should be allowed (window has elapsed)
	res, err = store.inc(key, limit, windowSize, lastRefill+500)
	if err != nil {
		t.Fatalf("request at lastRefill+500 failed: %v", err)
	}
	if res["s"] != success {
		t.Errorf("at lastRefill+500: expected success, got status=%d", res["s"])
	}
	if res["c"] != 1 {
		t.Errorf("at lastRefill+500: expected counter=1, got=%d", res["c"])
	}
	if res["ts"] != lastRefill+500 {
		t.Errorf("at lastRefill+500: expected ts=%d, got=%d", lastRefill+500, res["ts"])
	}
}

func TestRedigoSWStore_inc(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoSWStore(pool)

	// let's start with a key which doesn't exist yet
	key := "test"
	startTime := 1000
	now := startTime
	limit := 5
	windowSize := 1000 // i.e. 1s in millis
	success := 1
	fail := 0

	// clean up
	defer func() {
		pool.Get().Do("DEL", key)
	}()

	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != now {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], now)
		}
		if res["c"] != i {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], i)
		}
		if res["s"] != success {
			t.Errorf("invalid status got = %d, want = %d", res["s"], success)
		}
		now += 1
	}

	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != startTime {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], startTime)
		}
		if res["c"] != limit {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], i)
		}
		if res["s"] != fail {
			t.Errorf("invalid status got = %d, want = %d", res["s"], fail)
		}
		now = now + 1
	}
	//
	// lets increase the timestamp by 1000 and it should still fail
	now = 1990
	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != startTime {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], startTime)
		}
		if res["c"] != limit {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], limit)
		}
		if res["s"] != fail {
			t.Errorf("invalid status got = %d, want = %d", res["s"], fail)
		}
	}

	// lets elapse a second and now it should pass again
	now = 2500
	for i := 1; i <= limit; i++ {
		res, err := store.inc(key, limit, windowSize, now)
		if err != nil {
			t.Error("inc call failed = ", err)
		}
		if res["ts"] != now {
			t.Errorf("invalid ts got = %d, want = %d", res["ts"], now)
		}
		if res["c"] != i {
			t.Errorf("invalid counter got = %d, want = %d", res["c"], i)
		}
		if res["s"] != success {
			t.Errorf("invalid status got = %d, want = %d", res["s"], success)
		}
		now = now + 1
	}
}
