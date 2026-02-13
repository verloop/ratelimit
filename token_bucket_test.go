package ratelimit

import (
	"testing"
	"time"
)

func TestBucket_AllowWithStatus(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)
	rate := 5
	key := "test"

	n := time.Now()
	timeNow = func() time.Time {
		return n
	}

	b := &Bucket{
		Identifier: "marketing_campaigns",
		Rate:       rate,
		WindowSize: 1000,
		Store:      &store,
	}
	// nextRefresh := time.Duration(b.windowSize) * time.Millisecond
	for i := 1; i <= rate; i++ {
		got, err := b.AllowWithStatus(key)
		if err != nil {
			t.Error("AllowWithStatus call failed = ", err)
		}
		if !got.Allowed {
			t.Errorf("limit breached")
		}
		//if got.NextRefresh != nextRefresh {
		//	t.Errorf("invalid next refresh duration, got = %d, want = %d\n", got.NextRefresh, nextRefresh)
		//}
	}
	got, _ := b.AllowWithStatus(key)
	if got.Allowed {
		t.Error("expected to hit the limit")
	}
}

func TestBucket_AllowWithStatus_Rate10(t *testing.T) {
	pool := newRedisPool("localhost:6379")
	store := NewRedigoStore(pool)
	rate := 10
	key := "test_rate10_bucket"

	n := time.Now()
	timeNow = func() time.Time {
		return n
	}

	defer func() {
		pool.Get().Do("DEL", "rate10_test:"+key)
	}()

	b := &Bucket{
		Identifier: "rate10_test",
		Rate:       rate,
		WindowSize: 1000,
		Store:      &store,
	}

	// 10 requests should all be allowed
	for i := 1; i <= rate; i++ {
		got, err := b.AllowWithStatus(key)
		if err != nil {
			t.Fatalf("AllowWithStatus call %d failed: %v", i, err)
		}
		if !got.Allowed {
			t.Errorf("request %d: expected allowed, got denied", i)
		}
		expectedRemaining := rate - i
		if got.Remaining != expectedRemaining {
			t.Errorf("request %d: expected remaining=%d, got=%d", i, expectedRemaining, got.Remaining)
		}
	}

	// 11th request should be denied
	got, err := b.AllowWithStatus(key)
	if err != nil {
		t.Fatalf("AllowWithStatus call 11 failed: %v", err)
	}
	if got.Allowed {
		t.Error("request 11: expected denial, got allowed")
	}
	if got.Remaining != 0 {
		t.Errorf("request 11: expected remaining=0, got=%d", got.Remaining)
	}
}
