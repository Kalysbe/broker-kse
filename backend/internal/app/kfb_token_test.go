package app

import (
	"encoding/json"
	"statebroker/sbis/internal/adapters/kfb"
	"strings"
	"testing"
)

func TestProfileTokensArePreservedAndNotReturned(t *testing.T) {
	current := kfb.ConnectionSettings{Environment: "test", RESTURL: "http://host.docker.internal:8080", APIToken: "test-token"}
	profiles := defaultKFBProfiles(current)
	profiles["production"] = kfb.ConnectionSettings{Environment: "production", RESTURL: "http://172.23.10.84:8081", GRPCTarget: "172.23.10.84:8443", APIToken: "production-token"}
	input := profiles["production"]
	input.APIToken = ""
	saved, err := updateKFBProfile(input, current, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if saved.APIToken != "production-token" {
		t.Fatal("wrong profile token used")
	}
	raw, _ := json.Marshal(publicKFBProfile(saved))
	if strings.Contains(string(raw), "production-token") || strings.Contains(string(raw), "api_token") {
		t.Fatal("token exposed")
	}
	input.APIToken = "new\r\ntoken"
	if _, err := updateKFBProfile(input, current, profiles); err == nil {
		t.Fatal("invalid header token accepted")
	}
}

func TestTokenConnectionNeedsNoUploadedCertificates(t *testing.T) {
	settings := kfb.ConnectionSettings{RESTURL: "http://host.docker.internal:8080", GRPCTarget: "host.docker.internal:8443", APIToken: "token"}
	if err := kfb.ValidateConnection(settings); err != nil {
		t.Fatal(err)
	}
}
