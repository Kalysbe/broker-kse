package kfb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"statebroker/sbis/internal/adapters"
)

func TestGateConcurrentSpacing(t *testing.T) {
	g := &requestGate{interval: 40 * time.Millisecond}
	testSpacing(t, g, g)
}

func TestRedisGateSharedAcrossClients(t *testing.T) {
	address := os.Getenv("KFB_TEST_REDIS_URL")
	if address == "" {
		t.Skip("integration test requires Redis")
	}
	options, err := redis.ParseURL(address)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(options)
	defer rdb.Close()
	key := "kfb:test:" + t.Name() + time.Now().String()
	defer rdb.Del(context.Background(), key)
	a := &requestGate{redis: rdb, key: key, interval: 40 * time.Millisecond}
	b := &requestGate{redis: rdb, key: key, interval: 40 * time.Millisecond}
	testSpacing(t, a, b)
}

func testSpacing(t *testing.T, a, b *requestGate) {
	t.Helper()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var times []time.Time
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g := a
			if i%2 == 1 {
				g = b
			}
			if err := g.wait(ctx); err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			times = append(times, time.Now())
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	if len(times) != 6 {
		t.Fatal("missing requests")
	}
	for i := 1; i < len(times); i++ {
		if times[i].Sub(times[i-1]) < 30*time.Millisecond {
			t.Fatal("request burst exceeded budget")
		}
	}
	if err := a.pause(ctx, 80*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := b.wait(ctx); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 65*time.Millisecond {
		t.Fatal("shared cooldown was ignored")
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if err := a.wait(cancelled); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestSendOrderSkipsHealthProbeAndPreserves429(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v1/orders" {
			t.Error("unexpected preflight")
		}
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
	}))
	defer server.Close()
	c := New(server.URL, "secret")
	_, err := c.SendOrder(context.Background(), adapters.VenueOrder{ClientOrderID: "test"})
	if !isRateLimited(err) || calls != 1 {
		t.Fatalf("lost rate limit or extra calls: %v (%d)", err, calls)
	}
	if time.Until(c.gate.next) < time.Second {
		t.Fatal("Retry-After was ignored")
	}
}

func TestHealthCachesThrottleAsDegraded(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(429) }))
	defer server.Close()
	c := New(server.URL, "secret")
	for i := 0; i < 3; i++ {
		if health := c.HealthCheck(context.Background()); health != adapters.Degraded {
			t.Fatalf("throttle marked as %s", health)
		}
	}
	if calls != 1 {
		t.Fatalf("health probes were not cached: %d", calls)
	}
}

func TestTradingAndReadingBudgetsAreIndependent(t *testing.T) {
	c := New("http://example.invalid", "test")
	c.ConfigureRateLimit(nil, 20, 100)
	if c.gate.interval < 50*time.Millisecond || c.readGate.interval < 10*time.Millisecond {
		t.Fatal("configured limits exceeded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.gateFor(http.MethodPost).pause(ctx, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := c.gateFor(http.MethodGet).wait(ctx); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("trading cooldown blocked reading")
	}
	if c.gateFor(http.MethodPost) != c.gateFor(http.MethodDelete) || c.gateFor(http.MethodGet) == c.gateFor(http.MethodPost) {
		t.Fatal("incorrect request classification")
	}
}
