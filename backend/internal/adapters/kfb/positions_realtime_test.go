package kfb

import "testing"

func TestParseGRPCPositionsSnapshot(t *testing.T) {
	full := ParsePositions([]byte(`{"source":"QUIK","firm_id":"2040","cash":[{"kind":"CASH","client_code":"kalys","currency":"KGS","quantity":"1500000"}],"instruments":[{"client_code":"kalys","symbol":"ABC","quantity":"100"}]}`))
	if len(full.Positions) != 2 || full.Note != "QUIK" {
		t.Fatalf("unexpected full snapshot: %+v", full)
	}
	empty := ParsePositions([]byte(`{"source":"QUIK","cash":[],"instruments":[]}`))
	if empty.Positions == nil || len(empty.Positions) != 0 {
		t.Fatalf("empty snapshot must clear positions: %+v", empty)
	}
}
