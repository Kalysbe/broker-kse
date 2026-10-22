package kfb

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Read-only integration check against the configured mTLS gateway, no orders submitted.
func TestLiveNativeOrderSubscription(t *testing.T) {
	base := os.Getenv("KFB_REALTIME_TEST_URL")
	if base == "" {
		t.Skip("requires live gateway")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	acknowledged := false
	err := New(base, os.Getenv("KFB_GATEWAY_TOKEN")).StreamOrders(ctx, func() error { acknowledged = true; cancel(); return nil }, func(json.RawMessage) error { return nil })
	if !acknowledged {
		t.Fatalf("native subscription did not become ready: %v", err)
	}
}
