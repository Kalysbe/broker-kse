package kfb

import (
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

type ConnectionSettings struct {
	APIToken    string `json:"api_token,omitempty"`
	Environment string `json:"environment,omitempty"`
	RESTURL     string `json:"rest_url"`
	GRPCTarget  string `json:"grpc_target"`
	ServerName  string `json:"server_name"`
	CAPEM       string `json:"ca_pem,omitempty"`
	CertPEM     string `json:"cert_pem,omitempty"`
	KeyPEM      string `json:"key_pem,omitempty"`
}

func (c *Client) ConnectionSettings() ConnectionSettings {
	if settings := c.connection.Load(); settings != nil {
		copy := *settings
		if copy.APIToken == "" {
			copy.APIToken = c.token
		}
		return copy
	}
	ca, _ := os.ReadFile(os.Getenv("KFB_GRPC_CA"))
	return ConnectionSettings{RESTURL: c.base, GRPCTarget: os.Getenv("KFB_GRPC_TARGET"), ServerName: os.Getenv("KFB_GRPC_SERVER_NAME"), CAPEM: string(ca), APIToken: c.token}
}

func (c *Client) gatewayToken() string {
	if settings := c.connection.Load(); settings != nil && settings.APIToken != "" {
		return settings.APIToken
	}
	return c.token
}

func (c *Client) baseURL() string {
	if settings := c.connection.Load(); settings != nil {
		return settings.RESTURL
	}
	return c.base
}

func (c *Client) ApplyConnection(settings ConnectionSettings) {
	c.connection.Store(&settings)
	c.connectionVersion.Add(1)
	c.mu.Lock()
	c.last = Snapshot{}
	c.mu.Unlock()
}

func (c *Client) ConnectionVersion() uint64 { return c.connectionVersion.Load() }

func ValidateConnection(s ConnectionSettings) error {
	u, err := url.Parse(s.RESTURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("Укажите адрес REST сервера с портом, без /api/v1, например http://172.23.10.96:8081")
	}
	if _, port, err := net.SplitHostPort(s.GRPCTarget); err != nil || port == "" {
		return fmt.Errorf("Укажите адрес gRPC с портом, например 172.23.10.96:8443")
	}

	roots := x509.NewCertPool()
	if s.CAPEM != "" && !roots.AppendCertsFromPEM([]byte(s.CAPEM)) {
		return fmt.Errorf("Файл CA не содержит сертификат в формате PEM")
	}
	return nil
}

func NormalizeConnection(s ConnectionSettings) ConnectionSettings {
	s.RESTURL = strings.TrimRight(strings.TrimSpace(s.RESTURL), "/")
	s.RESTURL = strings.TrimSuffix(s.RESTURL, "/api/v1")
	s.APIToken = strings.TrimSpace(s.APIToken)
	s.GRPCTarget = strings.TrimSpace(s.GRPCTarget)
	s.ServerName = strings.TrimSpace(s.ServerName)
	return s
}
