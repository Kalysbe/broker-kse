package hash_test

import (
	"testing"

	"statebroker/sbis/internal/platform/hash"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := hash.Password("CorrectHorseBatteryStaple")
	if err != nil {
		t.Fatal(err)
	}
	if !hash.VerifyPassword("CorrectHorseBatteryStaple", h) {
		t.Fatal("verify failed")
	}
	if hash.VerifyPassword("wrong-password-xx", h) {
		t.Fatal("false positive")
	}
}

func TestTokenHashStable(t *testing.T) {
	a := hash.SHA256Hex("abc")
	b := hash.SHA256Hex("abc")
	if a != b || len(a) != 64 {
		t.Fatal(a, b)
	}
}
