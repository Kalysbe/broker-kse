// Package mobileupdates serves pre-signed Expo updates. Signing keys remain on
// the release operator's machine; the API only reads immutable release files.
package mobileupdates

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/go-chi/chi/v5"
)

type Handler struct{ Dir string }

var runtimeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,95}$`)
var assetName = regexp.MustCompile(`^[a-f0-9]{64}\.[a-z0-9]{1,10}$`)

type release struct {
	Manifest  json.RawMessage `json:"manifest"`
	Signature string          `json:"signature"`
}

func (h Handler) Manifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Expo-Protocol-Version", "1")
	w.Header().Set("Expo-Sfv-Version", "0")
	w.Header().Set("Vary", "Expo-Platform, Expo-Runtime-Version, Expo-Current-Update-Id, Accept")
	platform, runtime := r.Header.Get("Expo-Platform"), r.Header.Get("Expo-Runtime-Version")
	if (platform != "android" && platform != "ios") || !runtimeName.MatchString(runtime) {
		http.Error(w, "Invalid update platform or runtime", http.StatusBadRequest)
		return
	}
	if r.Header.Get("Expo-Protocol-Version") != "1" {
		http.Error(w, "Unsupported update protocol", http.StatusNotAcceptable)
		return
	}
	root, err := os.OpenRoot(h.Dir)
	if errors.Is(err, os.ErrNotExist) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		http.Error(w, "Updates unavailable", http.StatusServiceUnavailable)
		return
	}
	defer root.Close()
	file, err := root.Open(filepath.Join("current", runtime, platform+".json"))
	if errors.Is(err, os.ErrNotExist) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		http.Error(w, "Updates unavailable", http.StatusServiceUnavailable)
		return
	}
	defer file.Close()
	var update release
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	if err := decoder.Decode(&update); err != nil || !json.Valid(update.Manifest) {
		http.Error(w, "Invalid update release", http.StatusInternalServerError)
		return
	}
	var manifest struct {
		ID             string `json:"id"`
		RuntimeVersion string `json:"runtimeVersion"`
	}
	signature, err := base64.StdEncoding.DecodeString(update.Signature)
	if err != nil || len(signature) < 256 || len(signature) > 1024 || json.Unmarshal(update.Manifest, &manifest) != nil || manifest.ID == "" || manifest.RuntimeVersion != runtime {
		http.Error(w, "Invalid update release", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Expo-Current-Update-Id") == manifest.ID {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/expo+json; charset=utf-8")
	w.Header().Set("Expo-Signature", `sig="`+update.Signature+`", keyid="main", alg="rsa-v1_5-sha256"`)
	w.WriteHeader(http.StatusOK)
	// Never re-marshal: the signature authenticates these exact JSON bytes.
	_, _ = w.Write(update.Manifest)
}

func (h Handler) Asset(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !assetName.MatchString(key) {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(h.Dir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	file, err := root.Open(filepath.Join("assets", key))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(key))
	if filepath.Ext(key) == ".bundle" {
		contentType = "application/javascript"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, key, info.ModTime(), file)
}
