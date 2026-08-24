package gateway

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
	rps    float64
}

func NewLimiter() *Limiter {
	return &Limiter{buckets: map[string]*bucket{}}
}

func (l *Limiter) Allow(tokenID uuid.UUID, kind string, rps int) bool {
	if rps <= 0 {
		rps = 1
	}
	key := tokenID.String() + ":" + kind
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: float64(rps), last: now, rps: float64(rps)}
		l.buckets[key] = b
	}
	if b.rps != float64(rps) {
		b.rps = float64(rps)
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.rps
	if b.tokens > b.rps {
		b.tokens = b.rps
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
