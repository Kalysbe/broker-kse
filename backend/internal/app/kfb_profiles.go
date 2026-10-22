package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"statebroker/sbis/internal/adapters/kfb"
)

type kfbSettingsDocument struct {
	kfb.ConnectionSettings
	Profiles map[string]kfb.ConnectionSettings `json:"profiles,omitempty"`
}

func connectionEnvironment(settings kfb.ConnectionSettings) string {
	if settings.Environment != "" {
		return settings.Environment
	}
	u, _ := url.Parse(settings.RESTURL)
	if u != nil {
		switch u.Hostname() {
		case "host.docker.internal":
			return "test"
		case "172.23.10.84":
			return "production"
		}
	}
	return "custom"
}

func defaultKFBProfiles(current kfb.ConnectionSettings) map[string]kfb.ConnectionSettings {
	profiles := map[string]kfb.ConnectionSettings{
		"test":       {Environment: "test", RESTURL: "http://host.docker.internal:8080", GRPCTarget: "host.docker.internal:8443", ServerName: "localhost"},
		"production": {Environment: "production", RESTURL: "http://172.23.10.84:8081", GRPCTarget: "172.23.10.84:8443", ServerName: "172.23.10.84"},
	}
	current.Environment = connectionEnvironment(current)
	profiles[current.Environment] = current
	return profiles
}

func (a *App) loadKFBProfiles(ctx context.Context, current kfb.ConnectionSettings) (map[string]kfb.ConnectionSettings, error) {
	profiles := defaultKFBProfiles(current)
	var encrypted string
	err := a.DB.QueryRow(ctx, `SELECT value_json->>'encrypted' FROM system_settings WHERE key=$1`, kfbSettingsKey).Scan(&encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return profiles, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := openKFBSettings(a.Cfg.JWTSecret, encrypted)
	if err != nil {
		return nil, err
	}
	var document kfbSettingsDocument
	if err = json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	for name, settings := range document.Profiles {
		if name == "test" || name == "production" || name == "custom" {
			profiles[name] = settings
		}
	}
	profiles[connectionEnvironment(current)] = current
	return profiles, nil
}

func publicKFBProfile(settings kfb.ConnectionSettings) map[string]any {
	return map[string]any{"environment": connectionEnvironment(settings), "rest_url": settings.RESTURL, "grpc_target": settings.GRPCTarget, "server_name": settings.ServerName, "token_configured": settings.APIToken != ""}
}

func updateKFBProfile(input, current kfb.ConnectionSettings, profiles map[string]kfb.ConnectionSettings) (kfb.ConnectionSettings, error) {
	input = kfb.NormalizeConnection(input)
	if input.Environment == "" {
		input.Environment = connectionEnvironment(current)
	}
	previous, exists := profiles[input.Environment]
	if !exists {
		return input, fmt.Errorf("Выберите тестовый или боевой сервер")
	}
	expectedHost := map[string]string{"test": "host.docker.internal", "production": "172.23.10.84"}[input.Environment]
	if expectedHost != "" {
		rest, err := url.Parse(input.RESTURL)
		grpcHost, _, grpcErr := net.SplitHostPort(input.GRPCTarget)
		if err != nil || rest.Hostname() != expectedHost || grpcErr != nil || grpcHost != expectedHost {
			return input, fmt.Errorf("Для выбранного сервера используйте адрес %s в REST и gRPC", expectedHost)
		}
	}
	if input.APIToken == "" {
		input.APIToken = previous.APIToken
	}
	if len(input.APIToken) > 8192 || strings.ContainsAny(input.APIToken, "\r\n") {
		return input, fmt.Errorf("Некорректный токен КФБ")
	}
	if input.CAPEM == "" {
		input.CAPEM = previous.CAPEM
	}
	if input.ServerName == "" {
		input.ServerName = previous.ServerName
	}
	if input.CertPEM == "" {
		input.CertPEM = previous.CertPEM
	}
	if input.KeyPEM == "" {
		input.KeyPEM = previous.KeyPEM
	}
	if err := kfb.ValidateConnection(input); err != nil {
		return input, err
	}
	return input, nil
}
