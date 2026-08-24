package app

import "testing"

func TestParseKFBInstrument(t *testing.T) {
	got := parseKFBInstrument(map[string]any{
		"quik_symbol":    "KG0105166315",
		"internal_symbol": "KG0105166315",
		"board":          "LISTEDSTOCKK",
		"currency":       "KGS",
		"security_type":  "CS",
		"status":         "ACTIVE",
	})
	if got.Symbol != "KG0105166315" || got.Board != "LISTEDSTOCKK" || got.Currency != "KGS" {
		t.Fatalf("%+v", got)
	}
	if got.Status != "ACTIVE" {
		t.Fatalf("status %s", got.Status)
	}
}

func TestParseKFBInstrumentFallsBackToTicker(t *testing.T) {
	got := parseKFBInstrument(map[string]any{"ticker": "ABC", "board": "MAIN"})
	if got.Symbol != "ABC" || got.Board != "MAIN" || got.Currency != "KGS" {
		t.Fatalf("%+v", got)
	}
}

func TestOrderCanCancel(t *testing.T) {
	if !orderCanCancel("ACKNOWLEDGED") || !orderCanCancel("PARTIALLY_FILLED") || !orderCanCancel("PENDING") {
		t.Fatal("expected cancellable statuses")
	}
	if orderCanCancel("FILLED") || orderCanCancel("CANCELLED") || orderCanCancel("NEW") {
		t.Fatal("terminal/new must not cancel from UI as live")
	}
}

func TestMapInstrumentStatus(t *testing.T) {
	if mapInstrumentStatus("") != "ACTIVE" || mapInstrumentStatus("TRADING") != "ACTIVE" {
		t.Fatal("open statuses")
	}
	if mapInstrumentStatus("HALTED") != "HALTED" {
		t.Fatal("halted")
	}
}
