package app

import (
	"net/http"
	"testing"

	"statebroker/sbis/internal/orders"
	"statebroker/sbis/internal/platform/httpx"
)

func TestErrorEnvelopeCodes(t *testing.T) {
	if httpx.ErrInsufficientFunds.Code != "INSUFFICIENT_FUNDS" {
		t.Fatal(httpx.ErrInsufficientFunds.Code)
	}
	if httpx.ErrInsufficientFunds.Status != http.StatusUnprocessableEntity {
		t.Fatal(httpx.ErrInsufficientFunds.Status)
	}
}

func TestNormalizeClientCode(t *testing.T) {
	if _, err := normalizeClientCode("  "); err == nil {
		t.Fatal("empty client code must fail")
	}
	got, err := normalizeClientCode("  006  ")
	if err != nil || got != "006" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestFilledIsTerminal(t *testing.T) {
	if !orders.IsTerminal(orders.Filled) || orders.IsTerminal(orders.New) {
		t.Fatal("terminal flags")
	}
}
