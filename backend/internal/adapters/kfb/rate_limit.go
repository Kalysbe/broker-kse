package kfb

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type requestGate struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
	redis    *redis.Client
	key      string
}

// ConfigureRateLimit shares the budget across API and worker. Redis failures
// stop requests rather than allowing either process to exceed the budget.
func (c *Client) ConfigureRateLimit(rdb *redis.Client, tradingRPS, readRPS int) {
	if tradingRPS < 1 {
		tradingRPS = 20
	}
	if readRPS < 1 {
		readRPS = 100
	}
	key := fmt.Sprintf("kfb:rate:%x", sha256.Sum256([]byte(c.base)))
	c.gate = &requestGate{interval: time.Second/time.Duration(tradingRPS) + time.Millisecond, redis: rdb, key: key + ":trading"}
	c.readGate = &requestGate{interval: time.Second/time.Duration(readRPS) + time.Millisecond, redis: rdb, key: key + ":reading"}
}

func (c *Client) gateFor(method string) *requestGate {
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return c.readGate
	}
	return c.gate
}

var acquireSlot = redis.NewScript(`
local t=redis.call('TIME')
local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
local next=tonumber(redis.call('GET',KEYS[1]) or '0')
if next>now then return next-now end
redis.call('SET',KEYS[1],now+tonumber(ARGV[1]),'PX',tonumber(ARGV[1])+60000)
return 0
`)
var pauseSlots = redis.NewScript(`
local t=redis.call('TIME')
local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
local next=math.max(tonumber(redis.call('GET',KEYS[1]) or '0'),now+tonumber(ARGV[1]))
redis.call('SET',KEYS[1],next,'PX',next-now+60000)
return 0
`)

func (g *requestGate) wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var delay time.Duration
		if g.redis != nil {
			ms, err := acquireSlot.Run(ctx, g.redis, []string{g.key}, g.interval.Milliseconds()).Int64()
			if err != nil {
				return fmt.Errorf("shared KFB limiter: %w", err)
			}
			delay = time.Duration(ms) * time.Millisecond
		} else {
			g.mu.Lock()
			delay = time.Until(g.next)
			if delay <= 0 {
				g.next = time.Now().Add(g.interval)
			}
			g.mu.Unlock()
		}
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (g *requestGate) pause(ctx context.Context, delay time.Duration) error {
	if g.redis != nil {
		return pauseSlots.Run(ctx, g.redis, []string{g.key}, delay.Milliseconds()).Err()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	until := time.Now().Add(delay)
	if until.After(g.next) {
		g.next = until
	}
	return nil
}

func retryDelay(value string) time.Duration {
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && time.Until(at) > 0 {
		return time.Until(at)
	}
	return time.Second
}

func isRateLimited(err error) bool {
	var diagnostic *RequestError
	return errors.As(err, &diagnostic) && diagnostic.HTTPStatus == 429
}

func isThrottled(err error) bool {
	var diagnostic *RequestError
	return errors.As(err, &diagnostic) && (diagnostic.HTTPStatus == 429 || diagnostic.Stage == "rate_limit_wait")
}

type limitedTransport struct {
	client *Client
	base   http.RoundTripper
}

// LimitedTransport is for clients such as the direct-exchange load tester.
func (c *Client) LimitedTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return limitedTransport{client: c, base: base}
}
func (t limitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	gate := t.client.gateFor(req.Method)
	if err := gate.wait(req.Context()); err != nil {
		return nil, err
	}
	res, err := t.base.RoundTrip(req)
	if err == nil && res.StatusCode == 429 {
		if pauseErr := gate.pause(req.Context(), retryDelay(res.Header.Get("Retry-After"))); pauseErr != nil {
			res.Body.Close()
			return nil, pauseErr
		}
	}
	return res, err
}
