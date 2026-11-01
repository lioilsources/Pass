package httpapi

import (
	"sync"
	"time"
)

// limiter is a per-key token bucket: `burst` tokens, refilled at `rate` per
// minute. Keys that have been idle for an hour are dropped on the next sweep.
type limiter struct {
	perMinute float64
	burst     float64
	now       func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	sweptAt time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(perMinute int, now func() time.Time) *limiter {
	return &limiter{perMinute: float64(perMinute), burst: float64(perMinute), now: now, buckets: map[string]*bucket{}}
}

// allow consumes a token for key and reports whether one was available.
func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.sweptAt) > time.Hour {
		for k, b := range l.buckets {
			if now.Sub(b.last) > time.Hour {
				delete(l.buckets, k)
			}
		}
		l.sweptAt = now
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Minutes() * l.perMinute
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
