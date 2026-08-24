package gateway

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHMACRoundTrip(t *testing.T) {
	token := "sk_test_abc"
	ts := strconv.FormatInt(1710000000, 10)
	nonce := "n-1"
	body := `{"account_id":"1-3101-006"}`
	sig := Sign(token, ts, nonce, "POST", "/api/v1/orders", body)
	if err := Verify(token, ts, nonce, "POST", "/api/v1/orders", body, sig, time.Unix(1710000000, 0)); err != nil {
		t.Fatal(err)
	}
	if err := Verify(token, ts, nonce, "POST", "/api/v1/orders", body, "deadbeef", time.Unix(1710000000, 0)); err == nil {
		t.Fatal("expected bad signature")
	}
	if err := Verify(token, "1", nonce, "POST", "/api/v1/orders", body, sig, time.Unix(1710000000, 0)); err == nil {
		t.Fatal("expected window error")
	}
}

func TestHMACGetEmptyBody(t *testing.T) {
	token := "sk_live_x"
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	sig := Sign(token, ts, "nonce-2", "GET", "/api/v1/firm", "")
	if err := Verify(token, ts, "nonce-2", "GET", "/api/v1/firm", "", sig, now); err != nil {
		t.Fatal(err)
	}
}

func TestTokenPrefix(t *testing.T) {
	if TokenPrefix("sk_live_AbCdEfGhIj") != "sk_live_AbCd…" {
		t.Fatalf("got %s", TokenPrefix("sk_live_AbCdEfGhIj"))
	}
	if !IsAPIToken("sk_test_x") || !IsAPIToken("sk_live_x") || IsAPIToken("sbis_x") {
		t.Fatal("token prefix")
	}
}

func TestFlexNumber(t *testing.T) {
	var n Number
	if err := json.Unmarshal([]byte(`"400"`), &n); err != nil || n.String() != "400" {
		t.Fatal(n, err)
	}
	if err := json.Unmarshal([]byte(`1`), &n); err != nil || n.String() != "1" {
		t.Fatal(n, err)
	}
	d, err := n.Decimal()
	if err != nil || !d.IsPositive() {
		t.Fatal(d, err)
	}
}

func TestUnknownJSONField(t *testing.T) {
	var req struct {
		AccountID string `json:"account_id"`
	}
	dec := json.NewDecoder(strings.NewReader(`{"account_id":"1","broker_id":"x"}`))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestCanCancel(t *testing.T) {
	if !CanCancel("ACKNOWLEDGED") || !CanCancel("PARTIALLY_FILLED") {
		t.Fatal("should allow")
	}
	for _, s := range []string{"QUEUED", "SENT", "FILLED", "CANCELLED", "REJECTED", "FAILED"} {
		if CanCancel(s) {
			t.Fatalf("must not cancel from %s", s)
		}
	}
}

func TestPublicStatus(t *testing.T) {
	if PublicStatus("NEW") != "QUEUED" || PublicStatus("ACKNOWLEDGED") != "ACKNOWLEDGED" {
		t.Fatal(PublicStatus("NEW"))
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter()
	id := uuid.New()
	allowed := 0
	for i := 0; i < 5; i++ {
		if l.Allow(id, "all", 2) {
			allowed++
		}
	}
	if allowed < 2 || allowed > 3 {
		t.Fatalf("allowed=%d", allowed)
	}
}
