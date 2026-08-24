package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const HMACWindow = 5 * time.Minute

func Sign(token, timestamp, nonce, method, path, body string) string {
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(Payload(timestamp, nonce, method, path, body)))
	return hex.EncodeToString(mac.Sum(nil))
}

func Payload(timestamp, nonce, method, path, body string) string {
	return timestamp + "\n" + nonce + "\n" + strings.ToUpper(method) + "\n" + path + "\n" + body
}

func Verify(token, timestamp, nonce, method, path, body, signature string, now time.Time) error {
	if timestamp == "" || nonce == "" || signature == "" {
		return fmt.Errorf("missing HMAC headers")
	}
	sec, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	ts := time.Unix(sec, 0)
	delta := now.Sub(ts)
	if delta < 0 {
		delta = -delta
	}
	if delta > HMACWindow {
		return fmt.Errorf("timestamp outside ±5 minute window")
	}
	want := Sign(token, timestamp, nonce, method, path, body)
	got := strings.ToLower(strings.TrimSpace(signature))
	if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		return fmt.Errorf("bad signature")
	}
	return nil
}

func TokenPrefix(token string) string {
	if len(token) <= 12 {
		return token + "…"
	}
	return token[:12] + "…"
}

func IsAPIToken(raw string) bool {
	return strings.HasPrefix(raw, "sk_test_") || strings.HasPrefix(raw, "sk_live_")
}
