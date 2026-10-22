package app

import (
	"encoding/json"
	"statebroker/sbis/internal/adapters/kfb"
	"testing"
)

func TestKFBProfilesKeepCurrentConnection(t *testing.T) {
	current := kfb.ConnectionSettings{RESTURL: "http://172.23.10.96:8081", CAPEM: "old-ca"}
	profiles := defaultKFBProfiles(current)
	if profiles["test"].RESTURL != "http://host.docker.internal:8080" || profiles["production"].GRPCTarget != "172.23.10.84:8443" {
		t.Fatal("incorrect default addresses")
	}
	if profiles["custom"].RESTURL != current.RESTURL {
		t.Fatal("existing connection lost")
	}
	if profiles["production"].CAPEM != "" || profiles["test"].CAPEM != "" {
		t.Fatal("CA copied between unrelated servers")
	}
}

func TestKFBProfileSwitchDoesNotUseActiveCA(t *testing.T) {
	current := kfb.ConnectionSettings{Environment: "test", RESTURL: "http://host.docker.internal:8080", CAPEM: "test-only-ca"}
	profiles := defaultKFBProfiles(current)
	next, err := updateKFBProfile(profiles["production"], current, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if next.CAPEM != "" || next.Environment != "production" {
		t.Fatal("test CA leaked into production")
	}
	if profiles["test"].CAPEM != "test-only-ca" {
		t.Fatal("test profile modified")
	}
	wrongServer := profiles["production"]
	wrongServer.RESTURL = profiles["test"].RESTURL
	if _, err := updateKFBProfile(wrongServer, current, profiles); err == nil {
		t.Fatal("production label accepted a test server address")
	}
	if _, err := updateKFBProfile(kfb.ConnectionSettings{Environment: "unknown"}, current, profiles); err == nil {
		t.Fatal("unknown profile accepted")
	}
}

func TestKFBProfilesPersistAndKeepLegacyCompatibility(t *testing.T) {
	current := kfb.ConnectionSettings{Environment: "test", RESTURL: "http://host.docker.internal:8080"}
	document := kfbSettingsDocument{ConnectionSettings: current, Profiles: defaultKFBProfiles(current)}
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var active kfb.ConnectionSettings
	if err = json.Unmarshal(raw, &active); err != nil || active.RESTURL != current.RESTURL {
		t.Fatal("runtime settings format changed", err)
	}
	var restored kfbSettingsDocument
	if err = json.Unmarshal(raw, &restored); err != nil || restored.Profiles["production"].RESTURL != "http://172.23.10.84:8081" {
		t.Fatal("profiles lost", err)
	}
	public := publicKFBProfile(kfb.ConnectionSettings{Environment: "test", KeyPEM: "secret", CAPEM: "secret-ca"})
	if _, exists := public["key_pem"]; exists {
		t.Fatal("key exposed")
	}
	if _, exists := public["ca_pem"]; exists {
		t.Fatal("CA contents exposed")
	}
}
