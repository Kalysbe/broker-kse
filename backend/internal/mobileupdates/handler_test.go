package mobileupdates

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func updateRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/mobile/updates", nil)
	r.Header.Set("Expo-Protocol-Version", "1")
	r.Header.Set("Expo-Platform", "android")
	r.Header.Set("Expo-Runtime-Version", "native-1")
	r.Header.Set("Accept", "application/expo+json")
	return r
}

func TestSignedManifestPreservesExactBytes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"id":"63b1b2ac-5856-4f6e-99c6-d535ad8cfe8f", "runtimeVersion":"native-1"}`)
	digest := sha256.Sum256(manifest)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	current := filepath.Join(dir, "current", "native-1")
	if err := os.MkdirAll(current, 0755); err != nil {
		t.Fatal(err)
	}
	envelope := fmt.Sprintf(`{"manifest":%s,"signature":%q}`, manifest, base64.StdEncoding.EncodeToString(signature))
	if err := os.WriteFile(filepath.Join(current, "android.json"), []byte(envelope), 0644); err != nil {
		t.Fatal(err)
	}
	h := Handler{Dir: dir}
	w := httptest.NewRecorder()
	h.Manifest(w, updateRequest())
	if w.Code != http.StatusOK || w.Body.String() != string(manifest) {
		t.Fatalf("response %d: %s", w.Code, w.Body.String())
	}
	served := sha256.Sum256(w.Body.Bytes())
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, served[:], signature); err != nil {
		t.Fatal("served JSON invalidated signature", err)
	}
	if !strings.Contains(w.Header().Get("Expo-Signature"), base64.StdEncoding.EncodeToString(signature)) {
		t.Fatal("signature missing")
	}
	r := updateRequest()
	r.Header.Set("Expo-Current-Update-Id", "63b1b2ac-5856-4f6e-99c6-d535ad8cfe8f")
	w = httptest.NewRecorder()
	h.Manifest(w, r)
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatal("running release offered again")
	}
	r = updateRequest()
	r.Header.Set("Expo-Runtime-Version", "native-2")
	w = httptest.NewRecorder()
	h.Manifest(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatal("incompatible runtime received a release")
	}
}

func TestAbsentReleaseDoesNotBlockClients(t *testing.T) {
	h := Handler{Dir: filepath.Join(t.TempDir(), "not-created")}
	w := httptest.NewRecorder()
	h.Manifest(w, updateRequest())
	if w.Code != http.StatusNoContent || w.Header().Get("Expo-Protocol-Version") != "1" {
		t.Fatalf("status %d", w.Code)
	}
	for _, runtime := range []string{"", "../../etc", strings.Repeat("a", 100)} {
		r := updateRequest()
		r.Header.Set("Expo-Runtime-Version", runtime)
		w = httptest.NewRecorder()
		h.Manifest(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatal("invalid runtime accepted", runtime)
		}
	}
	r := updateRequest()
	r.Header.Set("Expo-Protocol-Version", "0")
	w = httptest.NewRecorder()
	h.Manifest(w, r)
	if w.Code != http.StatusNotAcceptable {
		t.Fatal("unsupported protocol accepted")
	}
}

func TestUnsignedOrMismatchedReleaseIsNotServed(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current", "native-1")
	if err := os.MkdirAll(current, 0755); err != nil {
		t.Fatal(err)
	}
	for _, envelope := range []string{
		`{"manifest":{"id":"test","runtimeVersion":"native-1"},"signature":""}`,
		fmt.Sprintf(`{"manifest":{"id":"test","runtimeVersion":"other"},"signature":%q}`, base64.StdEncoding.EncodeToString(make([]byte, 256))),
		`broken json`,
	} {
		if err := os.WriteFile(filepath.Join(current, "android.json"), []byte(envelope), 0644); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		(Handler{Dir: dir}).Manifest(w, updateRequest())
		if w.Code != http.StatusInternalServerError {
			t.Fatal("invalid release was served")
		}
	}
}

func TestAssetsAreConfinedAndSupportPartialDownloads(t *testing.T) {
	dir := t.TempDir()
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0755); err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64) + ".bundle"
	if err := os.WriteFile(filepath.Join(assets, key), []byte("0123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Get("/assets/{key}", (Handler{Dir: dir}).Asset)
	r := httptest.NewRequest(http.MethodGet, "/assets/"+key, nil)
	r.Header.Set("Range", "bytes=2-5")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusPartialContent || w.Body.String() != "2345" || w.Header().Get("Content-Length") != "4" {
		t.Fatalf("partial download %d: %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/private-key.pem", nil))
	if w.Code != http.StatusNotFound {
		t.Fatal("non-asset path accepted")
	}
	outside := filepath.Join(t.TempDir(), "outside.bundle")
	_ = os.WriteFile(outside, []byte("private"), 0644)
	link := strings.Repeat("b", 64) + ".bundle"
	if err := os.Symlink(outside, filepath.Join(assets, link)); err == nil {
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/"+link, nil))
		if w.Code != http.StatusNotFound {
			t.Fatal("asset symlink escaped release directory")
		}
	}
}
