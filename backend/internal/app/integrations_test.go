package app

import "testing"

func TestFilterPositionRowsByClientCode(t *testing.T) {
	items := []map[string]any{
		{"kind": "INSTRUMENT", "client_code": "imar", "account_id": "1-3101-2040", "symbol": "KG0105166315"},
		{"kind": "INSTRUMENT", "client_code": "kalys", "account_id": "1-3101-2040", "symbol": "KG0105166315"},
		{"kind": "CASH", "client_code": "imar", "account_id": "1-3101-2040", "symbol": "KGS"},
		{"kind": "CASH", "client_code": "KALYS", "account_id": "1-3101-2040", "symbol": "KGS"},
	}
	got := filterPositionRows(items, "kalys")
	if len(got) != 2 {
		t.Fatalf("want 2 rows for kalys, got %d", len(got))
	}
	for _, row := range got {
		code := positionClientCode(row)
		if code != "kalys" && code != "KALYS" {
			t.Fatalf("leaked row %#v", row)
		}
	}
}

func TestVenueAccountFromRowsPrefersInstrumentAccount(t *testing.T) {
	rows := []map[string]any{
		{"kind": "CASH", "client_code": "kalys", "account_id": "", "symbol": "EQTV"},
		{"kind": "INSTRUMENT", "client_code": "kalys", "account_id": "1-3101-2040", "symbol": "KG0105166315"},
		{"kind": "INSTRUMENT", "client_code": "imar", "account_id": "1-3101-006", "symbol": "KG0105166315"},
	}
	got := venueAccountFromRows(rows, "kalys")
	if got != "1-3101-2040" {
		t.Fatalf("want 1-3101-2040, got %q", got)
	}
	if venueAccountFromRows(rows, "imar") != "1-3101-006" {
		t.Fatalf("imar account")
	}
	if venueAccountFromRows(rows, "") != "" {
		t.Fatal("empty client code must not pick an account")
	}
	cashOnly := []map[string]any{
		{"kind": "CASH", "client_code": "kalys", "account_id": "1-3101-2040"},
	}
	if venueAccountFromRows(cashOnly, "kalys") != "1-3101-2040" {
		t.Fatal("cash account is used when no instrument row has one")
	}
}

func TestFilterPositionRowsEmptyCodeHidesAll(t *testing.T) {
	items := []map[string]any{
		{"kind": "CASH", "client_code": "imar"},
		{"kind": "CASH", "client_code": "kalys"},
	}
	got := filterPositionRows(items, "  ")
	if len(got) != 0 {
		t.Fatalf("empty client code must hide all rows, got %d", len(got))
	}
}
