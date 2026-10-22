package kfb

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testConnectionCertificate(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: raw}))
}

func TestConnectionSettingsValidation(t *testing.T) {
	cert, key := testConnectionCertificate(t)
	valid := ConnectionSettings{RESTURL: "http://172.23.10.96:8081", GRPCTarget: "172.23.10.96:8443", ServerName: "localhost", CAPEM: cert, CertPEM: cert, KeyPEM: key}
	valid.CertPEM = ""
	valid.KeyPEM = ""
	if err := ValidateConnection(valid); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ConnectionSettings){func(s *ConnectionSettings) { s.RESTURL = "file:///etc/passwd" }, func(s *ConnectionSettings) { s.GRPCTarget = "missing-port" }, func(s *ConnectionSettings) { s.CAPEM = "invalid" }} {
		copy := valid
		change(&copy)
		if ValidateConnection(copy) == nil {
			t.Fatal("invalid settings accepted")
		}
	}
	normalized := NormalizeConnection(ConnectionSettings{RESTURL: " http://172.23.10.96:8081/api/v1/ "})
	if normalized.RESTURL != valid.RESTURL {
		t.Fatal("API prefix not normalized", normalized.RESTURL)
	}
}

func TestApplyConnectionChangesRESTAddress(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer profile-token" {
			t.Error("token not preserved")
		}
		w.Write([]byte(`{"cash":[],"instruments":[]}`))
	}))
	defer server.Close()
	client := New("http://127.0.0.1:1", "test-token")
	client.ApplyConnection(ConnectionSettings{RESTURL: server.URL, APIToken: "profile-token"})
	if _, err := client.FetchPositions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || client.ConnectionVersion() != 1 {
		t.Fatal("new connection was not used")
	}
}
