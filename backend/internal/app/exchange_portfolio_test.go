package app

import "testing"

func TestExchangeRowMatches(t *testing.T) {
	row := map[string]any{"client_code": "006", "account_id": "A", "currency": "KGS"}
	if !exchangeRowMatches(row, "006", "A", "KGS") {
		t.Fatal("own position rejected")
	}
	for _, args := range [][3]string{{"007", "A", "KGS"}, {"", "A", "KGS"}, {"006", "B", "KGS"}, {"006", "A", "USD"}} {
		if exchangeRowMatches(row, args[0], args[1], args[2]) {
			t.Fatal("foreign position matched")
		}
	}
}
