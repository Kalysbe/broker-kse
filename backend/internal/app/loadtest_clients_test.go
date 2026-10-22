package app

import "testing"

func TestParticipantsUseConfirmedGatewayCodes(t *testing.T) {
	rows := []map[string]any{{"client_code": "Kalys", "account_id": "shared"}, {"client_code": "imar", "account_id": "shared"}, {"client_code": "Temirlan", "account_id": "shared"}, {"client_code": "Emil", "account_id": "shared"}}
	participants, err := loadParticipants(rows, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if len(participants) != 4 || participants[1].ClientCode != "imar" {
		t.Fatal("gateway client code was changed")
	}
	if _, err := loadParticipants(rows[:3], "shared"); err == nil {
		t.Fatal("missing client was silently omitted")
	}
	if _, err := loadParticipants(rows, "another-account"); err == nil {
		t.Fatal("unverified account accepted")
	}
}
