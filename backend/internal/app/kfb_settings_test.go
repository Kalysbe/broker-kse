package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"statebroker/sbis/internal/platform/httpx"
)

func TestKFBSettingsEncryption(t *testing.T) {
	plaintext := []byte(`{"key_pem":"private-key","rest_url":"http://localhost:8081"}`)
	sealed, err := sealKFBSettings("secret", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "private-key") {
		t.Fatal("private key was stored as plaintext")
	}
	data, err := openKFBSettings("secret", sealed)
	if err != nil || string(data) != string(plaintext) {
		t.Fatal("round trip failed", err)
	}
	if _, err := openKFBSettings("different-secret", sealed); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := openKFBSettings("secret", sealed[:len(sealed)-4]+"AAAA"); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestKFBSettingsRequireAdministrator(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		req := httptest.NewRequest(method, "/v1/integrations/kfb/settings", nil)
		req = req.WithContext(httpx.WithPrincipal(context.Background(), httpx.Principal{Roles: []string{"OPERATOR"}}))
		response := httptest.NewRecorder()
		(&App{}).handleKFBSettings(response, req)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s allowed non-admin: %d", method, response.Code)
		}
	}
}
